package process

import (
	"os"
	"runtime"
	"strings"

	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/require"
)

const ModuleName = "process"

type ProcessModule struct {
	runtime *goja.Runtime
}

var sensitiveKeywords = []string{
	"PASSWORD", "SECRET", "TOKEN", "KEY", "API_", "DB_", "AWS_", "GOOGLE_",
	"ENCRYPTION", "PRIVATE", "SSL", "HEROKU", "NETLIFY", "DOCKER", "SMTP",
	"MAILGUN", "NEW_RELIC", "SENTRY", "CREDENTIALS",
}

func isSensitive(key string) bool {
	key = strings.ToUpper(key)
	for _, keyword := range sensitiveKeywords {
		if strings.Contains(key, keyword) {
			return true
		}
	}
	return false
}

// buildEnvMap build a map[string]string from os.Environ()
func (p *ProcessModule) buildEnvMap(key string) goja.Value {
	env := make(map[string]string)
	for _, e := range os.Environ() {
		kv := strings.SplitN(e, "=", 2)
		if !isSensitive(kv[0]) {
			if key != "" {
				if key == kv[0] {
					return p.runtime.ToValue(kv[1])
				}
			} else {
				env[kv[0]] = kv[1]
			}
		}
	}
	return p.runtime.ToValue(env)
}

func (p *ProcessModule) buildArgs() goja.Value {
	filtered := make([]string, 0, len(os.Args))
	for _, arg := range os.Args {
		if !strings.Contains(arg, "secret") && !strings.Contains(arg, "key") {
			filtered = append(filtered, arg)
		} else {
			filtered = append(filtered, "***REDACTED***")
		}
	}
	return p.runtime.ToValue(filtered)
}

func (p *ProcessModule) buildGetpid() goja.Value {
	return p.runtime.ToValue(os.Getpid())
}

func (p *ProcessModule) buildPlatform() goja.Value {
	return p.runtime.ToValue(runtime.GOOS)
}

func (p *ProcessModule) buildVersion() goja.Value {
	return p.runtime.ToValue(runtime.Version())
}

func (p *ProcessModule) buildCwd() goja.Value {
	dir, _ := os.Getwd()
	return p.runtime.ToValue(dir)
}

func Require(runtime *goja.Runtime, module *goja.Object) {
	p := &ProcessModule{
		runtime: runtime,
	}
	o := module.Get("exports").(*goja.Object)
	o.Set("env", p.buildEnvMap)
	o.Set("argv", p.buildArgs)
	o.Set("pid", p.buildGetpid)
	o.Set("platform", p.buildPlatform)
	o.Set("version", p.buildVersion)
	o.Set("cwd", p.buildCwd)
}

func Enable(runtime *goja.Runtime) {
	runtime.Set(ModuleName, require.Require(runtime, ModuleName))
}

func init() {
	require.RegisterCoreModule(ModuleName, Require)
}
