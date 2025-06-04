package exporter

import (
	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/require"
)

const ModuleName = "exporter"

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

	}
}

func Enable(runtime *goja.Runtime) {
	runtime.Set(ModuleName, require.Require(runtime, ModuleName))
}

func init() {
	require.RegisterCoreModule(ModuleName, Require)
}
