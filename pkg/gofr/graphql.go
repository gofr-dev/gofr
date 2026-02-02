package gofr

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"

	"github.com/graphql-go/graphql"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"

	"gofr.dev/pkg/gofr/container"
)

var (
	errHandlerNotFunc = errors.New("is not a function")
)

const (
	twoInputs = 2
)

type graphQLManager struct {
	container   *container.Container
	queries     map[string]any
	mutations   map[string]any
	schema      graphql.Schema
	schemaBuilt bool
	tracer      trace.Tracer
	typeCache   map[reflect.Type]graphql.Output
}

func newGraphQLManager(c *container.Container) *graphQLManager {
	return &graphQLManager{
		container: c,
		queries:   make(map[string]any),
		mutations: make(map[string]any),
		tracer:    otel.Tracer("gofr-graphql"),
		typeCache: make(map[reflect.Type]graphql.Output),
	}
}

func (m *graphQLManager) RegisterQuery(name string, handler any) {
	m.queries[name] = handler
}

func (m *graphQLManager) RegisterMutation(name string, handler any) {
	m.mutations[name] = handler
}

func (m *graphQLManager) buildSchema() error {
	queryFields := graphql.Fields{}

	for name, handler := range m.queries {
		field, err := m.buildField(name, handler)
		if err != nil {
			return err
		}

		queryFields[name] = field
	}

	// Add special gofr health field
	queryFields["gofr"] = &graphql.Field{
		Type: m.getGofrType(),
		Resolve: func(p graphql.ResolveParams) (any, error) {
			return m.container.Health(p.Context), nil
		},
	}

	rootQuery := graphql.ObjectConfig{Name: "RootQuery", Fields: queryFields}
	schemaConfig := graphql.SchemaConfig{Query: graphql.NewObject(rootQuery)}

	if len(m.mutations) > 0 {
		mutationFields := graphql.Fields{}

		for name, handler := range m.mutations {
			field, err := m.buildField(name, handler)
			if err != nil {
				return err
			}

			mutationFields[name] = field
		}

		rootMutation := graphql.ObjectConfig{Name: "RootMutation", Fields: mutationFields}
		schemaConfig.Mutation = graphql.NewObject(rootMutation)
	}

	var err error

	m.schema, err = graphql.NewSchema(schemaConfig)
	if err != nil {
		return err
	}

	m.schemaBuilt = true

	return nil
}

func (m *graphQLManager) buildField(name string, handler any) (*graphql.Field, error) {
	t := reflect.TypeOf(handler)
	if t.Kind() != reflect.Func {
		return nil, fmt.Errorf("handler for %s %w", name, errHandlerNotFunc)
	}

	field := &graphql.Field{
		Type:    m.getGraphQLType(t.Out(0)),
		Resolve: m.getResolver(name, handler),
	}

	// If the handler has 2 inputs, the second one is the arguments struct.
	if t.NumIn() == twoInputs {
		field.Args = m.getGraphQLArgs(t.In(1))
	}

	return field, nil
}

func (m *graphQLManager) getGraphQLArgs(t reflect.Type) graphql.FieldConfigArgument {
	inputType := t

	if t.Kind() == reflect.Ptr {
		inputType = t.Elem()
	}

	if inputType.Kind() != reflect.Struct {
		return nil
	}

	args := graphql.FieldConfigArgument{}

	for i := 0; i < inputType.NumField(); i++ {
		field := inputType.Field(i)
		if field.PkgPath != "" {
			continue
		}

		tag := field.Tag.Get("json")
		name := strings.Split(tag, ",")[0]

		if name == "" || name == "-" {
			name = field.Name
		}

		args[name] = &graphql.ArgumentConfig{
			Type: m.getInputType(field.Type),
		}
	}

	return args
}

func (m *graphQLManager) getInputType(t reflect.Type) graphql.Input {
	inputType := t

	if t.Kind() == reflect.Ptr {
		inputType = t.Elem()
	}

	switch inputType.Kind() {
	case reflect.String:
		return graphql.String
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return graphql.Int
	case reflect.Float32, reflect.Float64:
		return graphql.Float
	case reflect.Bool:
		return graphql.Boolean
	case reflect.Slice, reflect.Array:
		return graphql.NewList(m.getInputType(inputType.Elem()))
	case reflect.Invalid, reflect.Uintptr, reflect.Complex64, reflect.Complex128, reflect.Chan,
		reflect.Func, reflect.Map, reflect.Pointer, reflect.Struct, reflect.UnsafePointer, reflect.Interface:
		return graphql.String
	default:
		return graphql.String
	}
}

func (*graphQLManager) getGofrType() *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name: "GofrInfo",
		Fields: graphql.Fields{
			"status":  &graphql.Field{Type: graphql.String},
			"name":    &graphql.Field{Type: graphql.String},
			"version": &graphql.Field{Type: graphql.String},
		},
	})
}

// getGraphQLType uses reflection to build the GraphQL schema from Go types.
// DEVELOPER NOTE: Unlike HTTP handlers which return 'any' for loose serialization,
// GraphQL requires strong types (structs/slices) at startup to build the schema contract.
// If 'any' is used, the engine cannot discover nested fields (id, name, etc.),
// and the client will be unable to perform sub-field queries.
func (m *graphQLManager) getGraphQLType(t reflect.Type) graphql.Output {
	itemType := t

	if t.Kind() == reflect.Ptr || t.Kind() == reflect.Pointer {
		itemType = t.Elem()
	}

	if gqlType, ok := m.typeCache[itemType]; ok {
		return gqlType
	}

	switch itemType.Kind() {
	case reflect.Slice, reflect.Array:
		return graphql.NewList(m.getGraphQLType(itemType.Elem()))
	case reflect.Struct:
		return m.getStructType(itemType)
	case reflect.String, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64, reflect.Bool, reflect.Interface:
		return m.getScalarType(itemType.Kind())
	case reflect.Invalid, reflect.Uintptr, reflect.Complex64, reflect.Complex128, reflect.Chan,
		reflect.Func, reflect.Map, reflect.Pointer, reflect.UnsafePointer:
		return graphql.String
	default:
		return graphql.String
	}
}

func (*graphQLManager) getScalarType(k reflect.Kind) graphql.Output {
	switch k {
	case reflect.String, reflect.Interface:
		return graphql.String
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return graphql.Int
	case reflect.Float32, reflect.Float64:
		return graphql.Float
	case reflect.Bool:
		return graphql.Boolean
	case reflect.Invalid, reflect.Uintptr, reflect.Complex64, reflect.Complex128, reflect.Chan,
		reflect.Func, reflect.Map, reflect.Pointer, reflect.UnsafePointer, reflect.Array,
		reflect.Slice, reflect.Struct:
		return graphql.String
	default:
		return graphql.String
	}
}

func (m *graphQLManager) getStructType(t reflect.Type) graphql.Output {
	obj := graphql.NewObject(graphql.ObjectConfig{
		Name:   t.Name(),
		Fields: graphql.Fields{},
	})

	m.typeCache[t] = obj

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.PkgPath != "" { // Skip unexported fields
			continue
		}

		tag := field.Tag.Get("json")
		name := strings.Split(tag, ",")[0]

		if name == "" || name == "-" {
			name = field.Name
		}

		obj.AddFieldConfig(name, &graphql.Field{
			Type: m.getGraphQLType(field.Type),
		})
	}

	return obj
}

func (m *graphQLManager) getResolver(name string, h any) graphql.FieldResolveFn {
	v := reflect.ValueOf(h)

	return func(p graphql.ResolveParams) (any, error) {
		ctx, span := m.tracer.Start(p.Context, "graphql-resolver-"+name)
		defer span.End()

		gReq := &graphQLRequest{ctx: ctx, params: p.Args}
		c := newContext(nil, gReq, m.container)

		c.Debugf("Executing GraphQL Resolver: %s, Args: %v", name, p.Args)

		// Prepare arguments for function call.
		args := []reflect.Value{reflect.ValueOf(c)}

		// If the handler expects arguments, inject them.
		if v.Type().NumIn() == twoInputs {
			argVal := reflect.New(v.Type().In(1))
			_ = c.Bind(argVal.Interface())
			args = append(args, argVal.Elem())
		}

		results := v.Call(args)

		err := results[1].Interface()
		if err != nil {
			c.Errorf("GraphQL Resolver %s failed: %v", name, err)

			return nil, err.(error)
		}

		return results[0].Interface(), nil
	}
}

func (m *graphQLManager) GetHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !m.schemaBuilt {
			if err := m.buildSchema(); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)

				return
			}
		}

		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/ui") {
			m.renderPlayground(w)

			return
		}

		m.handleGraphQLRequest(w, r)
	})
}

func (m *graphQLManager) handleGraphQLRequest(w http.ResponseWriter, r *http.Request) {
	var params struct {
		Query         string         `json:"query"`
		OperationName string         `json:"operationName"`
		Variables     map[string]any `json:"variables"`
	}

	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		params.Query = r.URL.Query().Get("query")
	}

	result := graphql.Do(graphql.Params{
		Schema:         m.schema,
		RequestString:  params.Query,
		VariableValues: params.Variables,
		OperationName:  params.OperationName,
		Context:        r.Context(),
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	_ = json.NewEncoder(w).Encode(result)
}

func (*graphQLManager) renderPlayground(w http.ResponseWriter) {
	html := `<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8" />
  <title>GoFr GraphQL Playground</title>
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/graphql-playground-react/build/static/css/index.css" />
  <link rel="shortcut icon" href="https://cdn.jsdelivr.net/npm/graphql-playground-react/build/favicon.png" />
  <script src="https://cdn.jsdelivr.net/npm/graphql-playground-react/build/static/js/middleware.js"></script>
</head>
<body>
  <div id="root"></div>
  <script>
    window.addEventListener('load', function (event) {
      GraphQLPlayground.init(document.getElementById('root'), {
        endpoint: '/graphql',
      })
    })
  </script>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)

	_, _ = fmt.Fprint(w, html)
}

type graphQLRequest struct {
	ctx    context.Context
	params map[string]any
}

func (r *graphQLRequest) Context() context.Context { return r.ctx }

func (r *graphQLRequest) Param(key string) string {
	if val, ok := r.params[key]; ok {
		return fmt.Sprintf("%v", val)
	}

	return ""
}

func (*graphQLRequest) PathParam(_ string) string { return "" }

func (*graphQLRequest) HostName() string { return "" }

func (*graphQLRequest) Params(_ string) []string { return nil }

func (r *graphQLRequest) Bind(i any) error {
	data, err := json.Marshal(r.params)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, i)
}
