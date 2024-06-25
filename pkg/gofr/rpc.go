package gofr

import (
	"fmt"
	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/logging"
	"net"
	"net/rpc"
	"reflect"
	"strconv"
)

type rpcServer struct {
	server *rpc.Server
	port   int
}

type rpcClient struct {
	client *rpc.Client
}

func newRPCServer(port int) *rpcServer {
	return &rpcServer{
		server: rpc.NewServer(),
		port:   port,
	}
}

func (c *rpcClient) Call(serviceMethod string, args interface{}, reply interface{}) error {
	return c.client.Call(serviceMethod, args, reply)
}

func (r *rpcServer) Run(c *container.Container) {
	addr := ":" + strconv.Itoa(r.port)

	c.Logger.Infof("starting RPC server at %s", addr)

	l, err := net.Listen("tcp", addr)
	if err != nil {
		c.Logger.Errorf("Error listening: %v", err)
		return
	}
	defer l.Close()

	r.server.Accept(l)
}

func (r *rpcServer) RegisterRPCService(impl interface{}) {
	implType := reflect.TypeOf(impl)
	implValue := reflect.ValueOf(impl)
	methods := map[string]reflect.Value{}

	for i := 0; i < implType.NumMethod(); i++ {
		method := implType.Method(i)
		methodType := method.Type

		// Check method signature: func (receiver) MethodName(ctx *Context, req Request, resp *Response) error
		if methodType.NumIn() != 4 || methodType.NumOut() != 1 || methodType.Out(0) != reflect.TypeOf((*error)(nil)).Elem() {
			continue
		}

		// Get method input types
		inReqType := methodType.In(2)
		inRespType := methodType.In(3)

		// Create a new method without context parameter
		newMethodType := reflect.FuncOf([]reflect.Type{implType, inReqType, inRespType}, []reflect.Type{methodType.Out(0)}, false)
		newMethod := reflect.MakeFunc(newMethodType, func(args []reflect.Value) (results []reflect.Value) {
			ctx := &Context{
				Container: &container.Container{
					Logger: logging.NewLogger(logging.INFO),
				},
			}

			results = method.Func.Call([]reflect.Value{implValue, reflect.ValueOf(ctx), args[1], args[2]})
			return
		})

		methods[method.Name] = newMethod
	}

	for name, method := range methods {
		err := r.server.RegisterName(implType.Elem().Name()+"."+name, method.Interface())
		if err != nil {
			fmt.Printf("Error registering method %s: %v\n", name, err)
		}
	}
}
