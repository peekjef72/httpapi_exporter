package exporter

import (
	"fmt"
	"time"

	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/require"
)

const ModuleName = "exporter"

type ModuleExporter struct {
	runtime *goja.Runtime
	module  *goja.Object
}

func (e *ModuleExporter) toDateFunc(formatStr, dateStr string, locationStr string) (goja.Value, error) {
	var (
		timeValue time.Time
		err       error
	)
	if formatStr == "" {
		formatStr = time.RFC3339
	}
	if dateStr == "" {
		return nil, fmt.Errorf("date string to parse is empty")
	}
	if locationStr != "" {
		location, err := time.LoadLocation(locationStr)
		if err != nil {
			return nil, fmt.Errorf("can't load location: %s", err.Error())
		}
		timeValue, err = time.ParseInLocation(formatStr, dateStr, location)
	} else {

		timeValue, err = time.Parse(formatStr, dateStr)
	}
	if err != nil {
		return nil, fmt.Errorf("can't parse date from string: %s", err.Error())
	}

	dateConstructorObj := e.runtime.Get("Date").ToObject(e.runtime)
	dateConstructor, ok := goja.AssertConstructor(dateConstructorObj)
	// dateConstructor, ok := goja.AssertFunction(e.runtime.Get("Date"))
	if !ok {
		return nil, fmt.Errorf("impossible to find the Date constructor")
	}

	dateObj, err := dateConstructor(nil, e.runtime.ToValue(timeValue.UnixMilli()))
	if err != nil {
		return nil, err
	}
	return dateObj, nil
}

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
		exporter := &ModuleExporter{
			runtime: runtime,
			module:  o,
		}
		o.Set("toDate", exporter.toDateFunc)
	}
}

func Enable(runtime *goja.Runtime) {
	runtime.Set(ModuleName, require.Require(runtime, ModuleName))
}

func init() {
	require.RegisterCoreModule(ModuleName, Require)
}
