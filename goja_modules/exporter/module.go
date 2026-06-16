package exporter

import (
	// "fmt"
	// "time"

	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/require"
)

const ModuleName = "exporter"

// type ModuleExporter struct {
// 	runtime *goja.Runtime
// 	module  *goja.Object
// }

// func js_toDate(format string, str string) (time.Time, error) {
// 	ret_t, err := time.ParseInLocation(format, str, time.Local)
// 	if err != nil {
// 		err := fmt.Errorf("can't parse date from string: %s", err.Error())
// 		return time.Now(), err
// 	}
// 	return ret_t, nil
// }

// func (e *ModuleExporter) Call(f func(string, string)) func(goja.FunctionCall) goja.Value {
// 	return func(call goja.FunctionCall) goja.Value {
// 		if toDate, ok := goja.AssertFunction(e.module.Get("test_func")); ok {
// 			ret, err := toDate(call.Arguments...)
// 			if err != nil {
// 				panic(err)
// 			}
// 			return e.runtime.NewDate(ret)
// 		} else {
// 			panic(e.runtime.NewTypeError("util.format is not a function"))
// 		}
// 		// return nil
// 	}
// }

type JSModExporterFunc interface {
	GetJSFuncMap() map[string]any
}
type defaultJSModExporterFunc struct {
	func_map map[string]any
}

var (
	m = &defaultJSModExporterFunc{
		func_map: make(map[string]any),
	}
)

func (e *defaultJSModExporterFunc) GetJSFuncMap() map[string]any {
	return e.func_map
}

func Require(runtime *goja.Runtime, module *goja.Object) {
	requireWithJSModFuncMap(m)(runtime, module)
}

func RequireWithJSModFuncMap(func_map JSModExporterFunc) require.ModuleLoader {
	return requireWithJSModFuncMap(func_map)
}

func requireWithJSModFuncMap(func_map JSModExporterFunc) require.ModuleLoader {

	return func(runtime *goja.Runtime, module *goja.Object) {
		o := module.Get("exports").(*goja.Object)
		for func_name, func_code := range func_map.GetJSFuncMap() {
			o.Set(func_name, func_code)
		}
		// exporter := &ModuleExporter{
		// 	runtime: runtime,
		// 	module:  o,
		// }
		// o.Set("test_func", exporter.Call(js_toDate()))
	}
}

func Enable(runtime *goja.Runtime) {
	runtime.Set(ModuleName, require.Require(runtime, ModuleName))
}

func init() {
	require.RegisterCoreModule(ModuleName, Require)
}
