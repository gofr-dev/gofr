package gofr

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
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
	gofrKey = "gofr"
	stringT = "String"
	idT     = "ID"
	intT    = "Int"
	floatT  = "Float"
	boolT   = "Boolean"
)

type graphQLManager struct {
	container *container.Container
	queries   map[string]Handler
	mutations map[string]Handler
	schema    graphql.Schema
	buildErr  error
	mu        sync.RWMutex
	tracer    trace.Tracer
	typeCache map[string]graphql.Output
}

func newGraphQLManager(c *container.Container) *graphQLManager {
	c.Metrics().NewCounter("gofr_graphql_operations_total", "total Number of GraphQL operations received")
	c.Metrics().NewCounter("gofr_graphql_error_total", "total Number of GraphQL operations that returned an error")
	c.Metrics().NewHistogram("gofr_graphql_request_duration", "execution time of GraphQL requests",
		0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1, 5, 10) //nolint:mnd // histogram buckets

	return &graphQLManager{
		container: c,
		queries:   make(map[string]Handler),
		mutations: make(map[string]Handler),
		tracer:    otel.Tracer("gofr-graphql"),
		typeCache: make(map[string]graphql.Output),
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

	// Add special gofr health field by default if it's not already there
	if _, ok := queryFields[gofrKey]; !ok {
		queryFields[gofrKey] = &graphql.Field{
			Type: graphql.NewObject(graphql.ObjectConfig{
				Name: "GofrHealthInfo",
				Fields: graphql.Fields{
					"status":  &graphql.Field{Type: graphql.String},
					"name":    &graphql.Field{Type: graphql.String},
					"version": &graphql.Field{Type: graphql.String},
				},
			}),
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return m.container.Health(p.Context), nil
			},
		}
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
			// Add special gofr health field if defined in schema but no custom handler registered
			if field.Name == gofrKey {
				handler = func(c *Context) (any, error) {
					return m.container.Health(c.Context), nil
				}
			} else if strings.HasPrefix(field.Name, "__") {
				// Skip internal GraphQL fields like __schema, __type, etc.
				continue
			} else {
				return nil, fmt.Errorf("%w: %s", errResolverMissing, field.Name)
			}
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
	case stringT, idT:
		return graphql.String
	case intT:
		return graphql.Int
	case floatT:
		return graphql.Float
	case boolT:
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
		m.container.Errorf("GraphQL Enum type not yet supported: %s", name)
		return graphql.String
	}

	m.container.Errorf("unsupported GraphQL input type: %s", name)

	return graphql.String // Fallback
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
	case stringT, idT:
		return graphql.String
	case intT:
		return graphql.Int
	case floatT:
		return graphql.Float
	case boolT:
		return graphql.Boolean
	default:
		return m.getCustomOutputType(name, schema)
	}
}

func (m *graphQLManager) getCustomOutputType(name string, schema *ast.Schema) graphql.Output {
	def, ok := schema.Types[name]
	if def != nil && def.Kind == ast.Enum {
		m.container.Errorf("GraphQL Enum type not yet supported: %s", name)
		return graphql.String
	}

	if !ok || def.Kind != ast.Object {
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

		gReq := &graphQLRequest{ctx: ctx, params: p.Args}
		c := newContext(nil, gReq, m.container)

		c.Debugf("Executing GraphQL Resolver: %s, Args: %v", name, p.Args)

		res, err := h(c)
		if err != nil {
			m.container.Errorf("GraphQL Resolver %s failed: %v", name, err)
			return nil, err
		}

		return res, nil
	}
}

func (m *graphQLManager) Handle(w http.ResponseWriter, r *http.Request) {
	if m.buildErr != nil {
		m.container.Errorf("GraphQL build error: %v", m.buildErr)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)

		_ = json.NewEncoder(w).Encode(map[string]any{
			"errors": []map[string]any{
				{"message": m.buildErr.Error()},
			},
		})

		return
	}

	if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/graphql/ui") {
		m.renderPlayground(w, r)
		return
	}

	m.handleGraphQLRequest(w, r)
}

func (m *graphQLManager) handleGraphQLRequest(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	var req struct {
		Query         string         `json:"query"`
		OperationName string         `json:"operationName"`
		Variables     map[string]any `json:"variables"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		m.container.Metrics().IncrementCounter(r.Context(), "gofr_graphql_error_total", "operation_name", "unknown", "type", "unknown")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)

		_ = json.NewEncoder(w).Encode(map[string]any{
			"errors": []map[string]any{
				{"message": "invalid JSON request body"},
			},
		})

		return
	}

	opName := req.OperationName
	if opName == "" {
		opName = "unknown"
	}

	// Detect operation type
	opType := "query"
	if astDoc, err := parser.ParseQuery(&ast.Source{Input: req.Query}); err == nil && len(astDoc.Operations) > 0 {
		opType = string(astDoc.Operations[0].Operation)
	}

	ctx, span := m.tracer.Start(r.Context(), "graphql-request")
	span.SetAttributes(attribute.String("graphql.operation_name", opName), attribute.String("graphql.operation_type", opType))

	defer func() {
		m.container.Metrics().RecordHistogram(ctx, "gofr_graphql_request_duration",
			time.Since(start).Seconds(), "operation_name", opName, "type", opType)
		span.End()
	}()

	m.container.Metrics().IncrementCounter(ctx, "gofr_graphql_operations_total", "operation_name", opName, "type", opType)

	result := graphql.Do(graphql.Params{
		Schema:         m.schema,
		RequestString:  req.Query,
		VariableValues: req.Variables,
		OperationName:  req.OperationName,
		Context:        ctx,
	})

	w.Header().Set("Content-Type", "application/json")

	// Custom Error and Status Code Logic
	if len(result.Errors) > 0 {
		m.container.Metrics().IncrementCounter(ctx, "gofr_graphql_error_total", "operation_name", opName, "type", opType)

		// If there are errors (possibly validation or resolver errors), we return 422 vs 200
		w.WriteHeader(http.StatusUnprocessableEntity)

		if result.Data != nil {
			m.container.Debugf("GraphQL result partially matched schema. Errors: %v", result.Errors)
		}
	} else {
		w.WriteHeader(http.StatusOK)
	}

	err := json.NewEncoder(w).Encode(result)
	if err != nil {
		m.container.Errorf("error encoding GraphQL response: %v", err)
	}
}

func (*graphQLManager) renderPlayground(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	_, _ = w.Write([]byte(graphiqlHTML))
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
    <style>
        body { height: 100%; margin: 0; width: 100%; overflow: hidden; }
        #graphiql { height: 100vh; }
    </style>
    <link rel="stylesheet" href="https://unpkg.com/graphiql/graphiql.min.css" />
</head>
<body>
    <div id="graphiql">Loading...</div>
    <script src="https://unpkg.com/react@17/umd/react.production.min.js"></script>
    <script src="https://unpkg.com/react-dom@17/umd/react-dom.production.min.js"></script>
    <script src="https://unpkg.com/graphiql/graphiql.min.js"></script>
    <script>
        const fetcher = GraphiQL.createFetcher({ url: '/graphql' });
        ReactDOM.render(
            React.createElement(GraphiQL, { fetcher: fetcher, defaultQuery: '{ hello }' }),
            document.getElementById('graphiql')
        );
    </script>
</body>
</html>`
