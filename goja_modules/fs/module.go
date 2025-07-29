package fs

import (
	"os"

	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/require"
)

const ModuleName = "fs"

type FsModule struct {
	runtime *goja.Runtime
}

func (f *FsModule) ReadFileSync(filename string) goja.Value {
	code_b, err := os.ReadFile(filename)
	if err != nil {
		panic(f.runtime.NewTypeError(err.Error()))
	}
	return f.runtime.ToValue(string(code_b))
}

func Require(runtime *goja.Runtime, module *goja.Object) {
	f := &FsModule{
		runtime: runtime,
	}
	o := module.Get("exports").(*goja.Object)
	o.Set("readFileSync", f.ReadFileSync)
}

func Enable(runtime *goja.Runtime) {
	runtime.Set(ModuleName, require.Require(runtime, ModuleName))
}

func init() {
	require.RegisterCoreModule(ModuleName, Require)
}
