package gofr

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/parser"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"gofr.dev/pkg/gofr/container"
)

var (
	errSchemaMissing = errors.New("GraphQL schema file missing: ./configs/schema.graphqls")

	errResolverMissing = errors.New("resolver missing for field")
)

const (
	graphqlString  = "String"
	graphqlID      = "ID"
	graphqlInt     = "Int"
	graphqlFloat   = "Float"
	graphqlBoolean = "Boolean"

	graphqlQuery    = "query"
	graphqlMutation = "mutation"
	graphqlSuccess  = "success"
	graphqlError    = "error"
	graphqlUnknown  = "unknown"
)

// GraphQLLog represents a logged GraphQL resolver execution.
type GraphQLLog struct {
	Resolver string `json:"resolver"`
	Type     string `json:"type"`
	Duration int64  `json:"duration"`
	Error    string `json:"error,omitempty"`
}

func (l *GraphQLLog) PrettyPrint(writer io.Writer) {
	opType := "GraphQL Query"
	if l.Type == graphqlMutation {
		opType = "GraphQL Mutation"
	}

	if l.Error != "" {
		fmt.Fprintf(writer, "\u001B[38;5;8m%s %s: %s \n", l.Resolver, opType, l.Error)

		return
	}

	fmt.Fprintf(writer, "\u001B[38;5;8m%s %8d\u001B[38;5;8mµs\u001B[0m %s \n",
		l.Resolver, l.Duration, opType)
}

type graphQLManager struct {
	container *container.Container
	queries   map[string]Handler
	mutations map[string]Handler
	schema    graphql.Schema
	mu        sync.RWMutex
	tracer    trace.Tracer
	typeCache map[string]graphql.Output
	enumCache map[string]*graphql.Enum
}

func newGraphQLManager(c *container.Container) *graphQLManager {
	// GraphQL metrics are registered here (not in registerFrameworkMetrics) because
	// GraphQL is an opt-in feature. Metrics are only created when resolvers are registered.
	c.Metrics().NewCounter("app_graphql_operations_total", "Total number of GraphQL operations received.")
	c.Metrics().NewCounter("app_graphql_error_total", "Total number of GraphQL operations that returned an error.")
	c.Metrics().NewHistogram("app_graphql_request_duration", "Response time of GraphQL requests in seconds.",
		.001, .003, .005, .01, .02, .03, .05, .1, .2, .3, .5, .75, 1, 2, 3, 5, 10, 30) //nolint:mnd // histogram buckets

	return &graphQLManager{
		container: c,
		queries:   make(map[string]Handler),
		mutations: make(map[string]Handler),
		tracer:    otel.Tracer("gofr-graphql"),
		typeCache: make(map[string]graphql.Output),
		enumCache: make(map[string]*graphql.Enum),
	}
}

func (m *graphQLManager) RegisterQuery(name string, handler Handler) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.queries[name] = handler
}

func (m *graphQLManager) RegisterMutation(name string, handler Handler) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.mutations[name] = handler
}

func (m *graphQLManager) buildSchema() error {
	schemaContent, err := os.ReadFile("./configs/schema.graphqls")
	if err != nil {
		if os.IsNotExist(err) {
			return errSchemaMissing
		}

		return err
	}

	// Parse SDL using gqlparser
	src := &ast.Source{
		Name:  "schema.graphqls",
		Input: string(schemaContent),
	}

	gqlSchema, gqlErr := gqlparser.LoadSchema(src)
	if gqlErr != nil {
		return fmt.Errorf("failed to parse schema: %w", gqlErr)
	}

	// Bridge gqlparser AST to graphql-go Schema
	queryFields, err := m.buildFields(gqlSchema.Query, m.queries, gqlSchema)
	if err != nil {
		return err
	}

	mutationFields, err := m.buildFields(gqlSchema.Mutation, m.mutations, gqlSchema)
	if err != nil {
		return err
	}

	schemaConfig := graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{
			Name:   "Query",
			Fields: queryFields,
		}),
	}

	if len(mutationFields) > 0 {
		schemaConfig.Mutation = graphql.NewObject(graphql.ObjectConfig{
			Name:   "Mutation",
			Fields: mutationFields,
		})
	}

	m.schema, err = graphql.NewSchema(schemaConfig)
	if err != nil {
		return fmt.Errorf("failed to build graphql-go schema: %w", err)
	}

	return nil
}

func (m *graphQLManager) buildFields(obj *ast.Definition, handlers map[string]Handler, schema *ast.Schema) (graphql.Fields, error) {
	fields := graphql.Fields{}

	if obj == nil {
		return fields, nil
	}

	for _, field := range obj.Fields {
		m.mu.RLock()

		handler, ok := handlers[field.Name]

		m.mu.RUnlock()

		if !ok {
			if strings.HasPrefix(field.Name, "__") {
				// Skip internal GraphQL fields like __schema, __type, etc.
				continue
			}

			return nil, fmt.Errorf("%w: %s", errResolverMissing, field.Name)
		}

		fields[field.Name] = &graphql.Field{
			Type:    m.mapType(field.Type, schema),
			Args:    m.mapArgs(field.Arguments, schema),
			Resolve: m.getResolver(field.Name, handler),
		}
	}

	return fields, nil
}

func (m *graphQLManager) mapArgs(args ast.ArgumentDefinitionList, schema *ast.Schema) graphql.FieldConfigArgument {
	res := graphql.FieldConfigArgument{}

	for _, arg := range args {
		res[arg.Name] = &graphql.ArgumentConfig{
			Type: m.mapInputType(arg.Type, schema),
		}
	}

	return res
}

func (m *graphQLManager) mapInputType(t *ast.Type, schema *ast.Schema) graphql.Input {
	var coreType graphql.Input

	coreType = m.getCoreInputType(t.Name(), schema)

	if t.Elem != nil {
		coreType = graphql.NewList(m.mapInputType(t.Elem, schema))
	}

	if t.NonNull {
		return graphql.NewNonNull(coreType)
	}

	return coreType
}

func (m *graphQLManager) getCoreInputType(name string, schema *ast.Schema) graphql.Input {
	switch name {
	case graphqlString, graphqlID:
		return graphql.String
	case graphqlInt:
		return graphql.Int
	case graphqlFloat:
		return graphql.Float
	case graphqlBoolean:
		return graphql.Boolean
	default:
		return m.getCustomInputType(name, schema)
	}
}

func (m *graphQLManager) getCustomInputType(name string, schema *ast.Schema) graphql.Input {
	def, ok := schema.Types[name]
	if ok && def.Kind == ast.InputObject {
		fields := graphql.InputObjectConfigFieldMap{}

		for _, f := range def.Fields {
			fields[f.Name] = &graphql.InputObjectFieldConfig{
				Type: m.mapInputType(f.Type, schema),
			}
		}

		return graphql.NewInputObject(graphql.InputObjectConfig{
			Name:   name,
			Fields: fields,
		})
	}

	if def != nil && def.Kind == ast.Enum {
		return m.getEnum(def)
	}

	m.container.Errorf("unsupported GraphQL input type: %s", name)

	return graphql.String // Fallback
}

func (m *graphQLManager) getEnum(def *ast.Definition) *graphql.Enum {
	if e, ok := m.enumCache[def.Name]; ok {
		return e
	}

	config := graphql.EnumConfig{
		Name:   def.Name,
		Values: graphql.EnumValueConfigMap{},
	}

	for _, val := range def.EnumValues {
		config.Values[val.Name] = &graphql.EnumValueConfig{
			Value: val.Name,
		}
	}

	e := graphql.NewEnum(config)
	m.enumCache[def.Name] = e

	return e
}

func (m *graphQLManager) mapType(t *ast.Type, schema *ast.Schema) graphql.Output {
	var coreType graphql.Output

	if t.Elem != nil {
		coreType = graphql.NewList(m.mapType(t.Elem, schema))
	} else if gqlType, ok := m.typeCache[t.Name()]; ok {
		coreType = gqlType
	} else {
		coreType = m.getCoreOutputType(t.Name(), schema)
	}

	if t.NonNull {
		return graphql.NewNonNull(coreType)
	}

	return coreType
}

func (m *graphQLManager) getCoreOutputType(name string, schema *ast.Schema) graphql.Output {
	switch name {
	case graphqlString, graphqlID:
		return graphql.String
	case graphqlInt:
		return graphql.Int
	case graphqlFloat:
		return graphql.Float
	case graphqlBoolean:
		return graphql.Boolean
	default:
		return m.getCustomOutputType(name, schema)
	}
}

func (m *graphQLManager) getCustomOutputType(name string, schema *ast.Schema) graphql.Output {
	def, ok := schema.Types[name]
	if def != nil && def.Kind == ast.Enum {
		return m.getEnum(def)
	}

	if !ok || def.Kind != ast.Object {
		m.container.Errorf("unsupported GraphQL output type: %s, defaulting to String", name)
		return graphql.String // Fallback
	}

	obj := graphql.NewObject(graphql.ObjectConfig{
		Name:   name,
		Fields: graphql.Fields{},
	})

	// Cache it immediately to avoid infinite recursion for circular references
	m.typeCache[name] = obj

	for _, f := range def.Fields {
		obj.AddFieldConfig(f.Name, &graphql.Field{
			Type: m.mapType(f.Type, schema),
		})
	}

	return obj
}

// getResolver binds a handler of type gofr.Handler to a GraphQL field.
// Arguments are accessed inside the handler via c.Bind().
func (m *graphQLManager) getResolver(name string, h Handler) graphql.FieldResolveFn {
	return func(p graphql.ResolveParams) (any, error) {
		ctx, span := m.tracer.Start(p.Context, "graphql-resolver-"+name)
		defer span.End()

		start := time.Now()

		gReq := &graphQLRequest{ctx: ctx, params: p.Args}
		c := newContext(noopResponder{}, gReq, m.container)

		res, err := h(c)
		duration := time.Since(start).Microseconds()

		if err != nil {
			c.Error(&GraphQLLog{Resolver: name, Error: err.Error()})
			return nil, err
		}

		c.Debug(&GraphQLLog{Resolver: name, Type: m.getResolverType(name), Duration: duration})

		return res, nil
	}
}

func (m *graphQLManager) getResolverType(name string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, ok := m.mutations[name]; ok {
		return graphqlMutation
	}

	return graphqlQuery
}

const maxRequestBodySize = 32 << 20 // 32 MB

func (m *graphQLManager) Handle(w http.ResponseWriter, r *http.Request) {
	// Standard request protection - addressing review point 6 (bypassing middleware benefits)
	// Apply body size limit (using 32MB default as per GoFr's multipart decoder)
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)

	// Standard panic recovery for raw HTTP handler
	defer func() {
		if re := recover(); re != nil {
			m.container.Errorf("GraphQL Panic: %v\n%s", re, string(debug.Stack()))
			m.respondWithErrors(w, http.StatusInternalServerError, "Internal Server Error")
		}
	}()

	m.handleGraphQLRequest(w, r)
}

func (m *graphQLManager) handleGraphQLRequest(w http.ResponseWriter, r *http.Request) {
	ct := r.Header.Get("Content-Type")
	if ct != "" && !strings.HasPrefix(ct, "application/json") {
		m.respondWithErrors(w, http.StatusUnsupportedMediaType, "Content-Type must be application/json")
		return
	}

	start := time.Now()

	req, err := m.parseGraphQLRequest(r)
	if err != nil {
		m.container.Metrics().IncrementCounter(r.Context(), "app_graphql_error_total", "operation_name", graphqlUnknown, "type", graphqlUnknown)
		m.respondWithErrors(w, http.StatusBadRequest, "invalid JSON request body")

		return
	}

	opName := req.OperationName
	if opName == "" {
		opName = graphqlUnknown
	}

	opType := m.getOperationType(req.Query)

	ctx, span := m.tracer.Start(r.Context(), "graphql-request")
	span.SetAttributes(attribute.String("graphql.operation_name", opName), attribute.String("graphql.operation_type", opType))

	var result *graphql.Result

	defer func() {
		status := graphqlSuccess
		if result != nil && len(result.Errors) > 0 {
			status = graphqlError
		}

		m.container.Metrics().RecordHistogram(ctx, "app_graphql_request_duration",
			time.Since(start).Seconds(), "operation_name", opName, "type", opType, "status", status)
		span.End()
	}()

	m.container.Metrics().IncrementCounter(ctx, "app_graphql_operations_total", "operation_name", opName, "type", opType)

	result = graphql.Do(graphql.Params{
		Schema:         m.schema,
		RequestString:  req.Query,
		VariableValues: req.Variables,
		OperationName:  req.OperationName,
		Context:        ctx,
	})

	w.Header().Set("Content-Type", "application/json")

	if len(result.Errors) > 0 {
		m.container.Metrics().IncrementCounter(ctx, "app_graphql_error_total", "operation_name", opName, "type", opType)
	}

	w.WriteHeader(http.StatusOK)

	err = json.NewEncoder(w).Encode(result)
	if err != nil {
		m.container.Errorf("error encoding GraphQL response: %v", err)
	}
}

type gqlRequest struct {
	Query         string         `json:"query"`
	OperationName string         `json:"operationName"`
	Variables     map[string]any `json:"variables"`
}

func (*graphQLManager) parseGraphQLRequest(r *http.Request) (gqlRequest, error) {
	var req gqlRequest

	err := json.NewDecoder(r.Body).Decode(&req)

	return req, err
}

func (*graphQLManager) getOperationType(query string) string {
	opType := graphqlQuery
	if astDoc, err := parser.ParseQuery(&ast.Source{Input: query}); err == nil && len(astDoc.Operations) > 0 {
		opType = string(astDoc.Operations[0].Operation)
	}

	return opType
}

func (*graphQLManager) respondWithErrors(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(map[string]any{
		"errors": []map[string]any{
			{"message": message},
		},
	})
}
func (m *graphQLManager) GetHandler() http.Handler {
	return http.HandlerFunc(m.Handle)
}

// graphQLRequest implements the gofr.Request interface for GraphQL.
type graphQLRequest struct {
	ctx    context.Context
	params map[string]any
}

func (r *graphQLRequest) Param(name string) string {
	if v, ok := r.params[name]; ok {
		return fmt.Sprintf("%v", v)
	}

	return ""
}

func (*graphQLRequest) PathParam(string) string { return "" }

func (r *graphQLRequest) Bind(v any) error {
	b, err := json.Marshal(r.params)
	if err != nil {
		return err
	}

	return json.Unmarshal(b, v)
}

func (r *graphQLRequest) Context() context.Context { return r.ctx }
func (*graphQLRequest) HostName() string           { return "" }
func (*graphQLRequest) Params(string) []string     { return nil }

const graphiqlHTML = `<!DOCTYPE html>
<html lang="en">
<head>
    <title>GoFr GraphQL Playground</title>
    <link href="https://unpkg.com/graphiql@3.0.6/graphiql.min.css" rel="stylesheet" />
</head>
<body style="margin: 0;">
    <div id="graphiql" style="height: 100vh;"></div>

    <script crossorigin src="https://unpkg.com/react@18.2.0/umd/react.production.min.js"></script>
    <script crossorigin src="https://unpkg.com/react-dom@18.2.0/umd/react-dom.production.min.js"></script>
    <script crossorigin src="https://unpkg.com/graphiql@3.0.6/graphiql.min.js"></script>

    <script>
        const fetcher = GraphiQL.makeDefaultFetcher({ url: window.location.origin + '/graphql' });

        ReactDOM.render(
            React.createElement(GraphiQL, {
                fetcher: fetcher,
                defaultVariableEditorOpen: true,
                headerEditorEnabled: true,
                shouldPersistHeaders: true
            }),
            document.getElementById('graphiql'),
        );
    </script>
</body>
</html>`
