package dns

import (
	"net"
	"strconv"

	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/require"
)

const ModuleName = "dns"

type DNSModule struct {
	runtime       *goja.Runtime
	eventHandlers map[string][]goja.Callable
}

func (d *DNSModule) Lookup(call goja.FunctionCall) goja.Value {
	hostname := call.Argument(0)
	if goja.IsUndefined(hostname) && !goja.IsString(hostname) {
		panic(d.runtime.ToValue("argument must be a string"))
	}
	options := call.Argument(1)
	all := false
	family := 0
	if !goja.IsUndefined(options) {
		opts := options.ToObject(d.runtime)
		all = opts.Get("all").ToBoolean()
		family = int(opts.Get("family").ToInteger())
	}
	ips, err := net.LookupIP(hostname.String())
	if err != nil {
		// d.emit("error", err)
		return goja.Undefined()
	}
	ipList := d.runtime.NewArray()

	var res *goja.Object

	for idx, ip := range ips {
		ip_v4 := ip.To4()
		if (family == 4 && ip_v4 == nil) || (family == 6 && ip_v4 != nil) {
			continue
		}
		ipObj := d.runtime.NewObject()
		if ip_v4 != nil {
			ipObj.Set("family", 4)
			ipObj.Set("address", ip.To4().String())
		} else {
			ipObj.Set("family", 6)
			ipObj.Set("address", ip.String())
		}
		if !all {
			res = ipObj
			break
		} else {
			ipList.Set(strconv.Itoa(idx), ipObj)
		}
	}

	return res
}

func (d *DNSModule) On(call goja.FunctionCall) goja.Value {
	event := call.Argument(0).String()
	fn, ok := goja.AssertFunction(call.Argument(1))
	if !ok {
		panic(d.runtime.ToValue("Callback must be a function"))
	}
	d.eventHandlers[event] = append(d.eventHandlers[event], fn)
	return goja.Undefined()
}

func (d *DNSModule) Once(call goja.FunctionCall) goja.Value {
	event := call.Argument(0).String()
	cb := call.Argument(1).ToObject(d.runtime)
	fn, ok := goja.AssertFunction(cb)
	if !ok {
		panic(d.runtime.ToValue("Callback must be a function"))
	}

	// Store under event name
	d.eventHandlers[event] = append(d.eventHandlers[event], fn)
	return goja.Undefined()
}

func (d *DNSModule) RemoveListener(call goja.FunctionCall) goja.Value {
	event := call.Argument(0).String()
	// Just clear all for simplicity here
	delete(d.eventHandlers, event)
	return goja.Undefined()
}

func Require(runtime *goja.Runtime, module *goja.Object) {
	d := &DNSModule{
		runtime: runtime,
	}
	o := module.Get("exports").(*goja.Object)
	o.Set("lookup", d.Lookup)
}

func Enable(runtime *goja.Runtime) {
	runtime.Set(ModuleName, require.Require(runtime, ModuleName))
}

func init() {
	require.RegisterCoreModule(ModuleName, Require)
}
