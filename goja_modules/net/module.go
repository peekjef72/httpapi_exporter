package os

import (
	"net"
	"strconv"
	"time"

	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/require"
)

const ModuleName = "net"

type Socket struct {
	runtime       *goja.Runtime
	conn          net.Conn
	eventHandlers map[string][]goja.Callable
}

func (s *Socket) emit(event string, args ...goja.Value) {
	if handlers, ok := s.eventHandlers[event]; ok {
		for _, handler := range handlers {
			handler(nil, args...)
		}
	}
}
func (s *Socket) On(call goja.FunctionCall) goja.Value {
	event := call.Argument(0).String()
	fn, ok := goja.AssertFunction(call.Argument(1))
	if !ok {
		panic(s.runtime.ToValue("Callback must be a function"))
	}
	s.eventHandlers[event] = append(s.eventHandlers[event], fn)
	return goja.Undefined()
}

func (s *Socket) Once(call goja.FunctionCall) goja.Value {
	event := call.Argument(0).String()
	cb := call.Argument(1).ToObject(s.runtime)
	fn, ok := goja.AssertFunction(cb)
	if !ok {
		panic(s.runtime.ToValue("Callback must be a function"))
	}

	// Store under event name
	s.eventHandlers[event] = append(s.eventHandlers[event], fn)
	return goja.Undefined()
}

func (s *Socket) RemoveListener(call goja.FunctionCall) goja.Value {
	event := call.Argument(0).String()
	// Just clear all for simplicity here
	delete(s.eventHandlers, event)
	return goja.Undefined()
}

func (s *Socket) Destroy(goja.FunctionCall) goja.Value {
	if s.conn != nil {
		s.conn.Close()
	}
	return goja.Undefined()
}

func (s *Socket) Address(goja.FunctionCall) goja.Value {
	if s.conn == nil {
		return goja.Undefined()
	}
	addr := s.conn.LocalAddr()
	tcpAddr, ok := addr.(*net.TCPAddr)
	if !ok {
		return s.runtime.ToValue(addr.String())
	}
	obj := s.runtime.NewObject()
	obj.Set("address", tcpAddr.IP.String())
	obj.Set("port", tcpAddr.Port)
	if tcpAddr.IP.To4() != nil {
		obj.Set("family", "IPv4")
	} else {
		obj.Set("family", "IPv6")
	}
	return obj
}

type NetModule struct {
	runtime *goja.Runtime
}

// func (n *NetModule)Connect(host string, port int) (net.Conn, error) {
func (n *NetModule) Connect(call goja.FunctionCall) goja.Value {

	var (
		host string
		port int
	)

	if arg := call.Argument(0); !goja.IsUndefined(arg) {
		obj := arg.ToObject(n.runtime)
		port = int(obj.Get("port").ToInteger())
		host = obj.Get("host").ToString().String()
	} else {
		panic(n.runtime.NewTypeError("invalid parameter type"))
	}

	socket := &Socket{
		runtime:       n.runtime,
		eventHandlers: make(map[string][]goja.Callable),
	}

	go func() {
		conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, strconv.Itoa(port)), 2*time.Second)
		if err != nil {
			socket.emit("error")
			return
		}
		socket.conn = conn
		socket.emit("connect")
	}()

	socketObj := n.runtime.NewObject()
	socketObj.Set("once", socket.Once)
	socketObj.Set("removeListener", socket.RemoveListener)
	socketObj.Set("destroy", socket.Destroy)
	socketObj.Set("address", socket.Address)

	return socketObj
}

func Require(runtime *goja.Runtime, module *goja.Object) {
	n := &NetModule{
		runtime: runtime,
	}
	o := module.Get("exports").(*goja.Object)
	o.Set("connect", n.Connect)
}

func Enable(runtime *goja.Runtime) {
	runtime.Set(ModuleName, require.Require(runtime, ModuleName))
}

func init() {
	require.RegisterCoreModule(ModuleName, Require)
}
