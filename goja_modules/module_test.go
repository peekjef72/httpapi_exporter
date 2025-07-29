// cspell:ignore stretchr, svclb, svcgrplb, mypassword

package goja_modules

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/peekjef72/httpapi_exporter/template"
)

func TestJSModuleVarDefined(t *testing.T) {

	code := `
		var ret = false
		if( undef_var == undefined ) {
			ret = true
		}
		ret
	`

	js, err := NewJSCode(code, nil)
	if err != nil {
		assert.Nil(t, err, fmt.Sprintf(`TestJSModuleVarDefined compilation error: %s`, err.Error()))
		return
	}

	symtab := make(map[string]any)
	symtab["undef_var"] = nil
	v, err := js.Run(symtab, nil)
	if err != nil {
		assert.Nil(t, err, fmt.Sprintf(`TestJSModuleVarDefined compilation error: %s`, err.Error()))
		return
	}
	if value, ok := v.(bool); !ok {
		assert.True(t, ok, fmt.Sprintf("TestJSModuleVarDefined exec return value type differs: %v", v))
	} else {
		assert.True(t, value, fmt.Sprintf("TestJSModuleVarDefined exec return invalid value: %v", value))
	}
}

func TestJSModuleVarExists(t *testing.T) {

	code := `
		var ret = false
		/* try {
			if( undef_var == undefined ) {
				ret = true
			}
		}
		catch (err) {
		}
		function exists(my_var) {
			var ret = false
			if( typeof undef_var === 'undefined' || (typeof undef_var !== 'undefined' && undef_var == undefined ) ) {
				ret = true
			}
			return ret
		}

		exists( undef_var )
		*/
		var ret = false
		if( typeof undef_var === 'undefined' || (typeof undef_var !== 'undefined' && undef_var == undefined ) ) {
			ret = true
		}
		ret
	`

	js, err := NewJSCode(code, template.Js_func_map())
	if err != nil {
		assert.Nil(t, err, fmt.Sprintf(`TestJSModuleVarExists compilation error: %s`, err.Error()))
		return
	}

	symtab := make(map[string]any)
	// symtab["undef_var"] = nil
	v, err := js.Run(symtab, nil)
	if err != nil {
		assert.Nil(t, err, fmt.Sprintf(`TestJSModuleVarExists execution error: %s`, err.Error()))
		return
	}
	if value, ok := v.(bool); !ok {
		assert.True(t, ok, fmt.Sprintf("TestJSModuleVarExists exec return value type differs: %v", v))
	} else {
		assert.True(t, value, fmt.Sprintf("TestJSModuleVarExists exec return invalid value: %v", value))
	}

	symtab["undef_var"] = nil
	v, err = js.Run(symtab, nil)
	if err != nil {
		assert.Nil(t, err, fmt.Sprintf(`TestJSModuleVarExists execution error: %s`, err.Error()))
		return
	}
	if value, ok := v.(bool); !ok {
		assert.True(t, ok, fmt.Sprintf("TestJSModuleVarExists exec return value type differs: %v", v))
	} else {
		assert.True(t, value, fmt.Sprintf("TestJSModuleVarExists exec return invalid value: %v", value))
	}

}

func TestJSModuleCondComplex(t *testing.T) {

	code := `
	typeof config !== 'undefined' && 
  		config.ts_next_check <= Math.floor(new Date().getTime() / 1000)
`
	js, err := NewJSCode(code, template.Js_func_map())
	if err != nil {
		assert.Nil(t, err, fmt.Sprintf(`TestJSModuleCondComplex compilation error: %s`, err.Error()))
		return
	}

	symtab := make(map[string]any)

	config := make(map[string]any)
	config["ts_next_check"] = 0
	symtab["config"] = config

	v, err := js.Run(symtab, nil)
	if err != nil {
		assert.Nil(t, err, fmt.Sprintf(`TestJSModuleCondComplex compilation error: %s`, err.Error()))
		return
	}
	if value, ok := v.(bool); !ok {
		assert.True(t, ok, fmt.Sprintf("TestJSModuleCondComplex exec return value type differs: %v", v))
	} else {
		assert.True(t, value, fmt.Sprintf("TestJSModuleCondComplex exec return invalid value: %v", value))
	}

	code = `config.svclb = {}; config.svcgrplb = {}; true`

	js, err = NewJSCode(code, template.Js_func_map())
	if err != nil {
		assert.Nil(t, err, fmt.Sprintf(`TestJSModuleCondComplex compilation error: %s`, err.Error()))
		return
	}

	v, err = js.Run(symtab, nil)
	if err != nil {
		assert.Nil(t, err, fmt.Sprintf(`TestJSModuleCondComplex compilation error: %s`, err.Error()))
		return
	}
	if value, ok := v.(bool); !ok {
		assert.True(t, ok, fmt.Sprintf("TestJSModuleCondComplex exec return value type differs: %v", v))
	} else {
		assert.True(t, value, fmt.Sprintf("TestJSModuleCondComplex exec return invalid value: %v", value))

		_, ok := config["svclb"]
		assert.True(t, ok, fmt.Sprintf("TestJSModuleCondComplex exec return invalid value: %v", value))

		_, ok = config["svcgrplb"]
		assert.True(t, ok, fmt.Sprintf("TestJSModuleCondComplex exec return invalid value: %v", value))
	}

}

func TestJSModuleAnalyzeComplex(t *testing.T) {

	code := `
		var status = -1;
		if( results.length > 0 ) {
			// console.info("nb of line: " + results.length);
			
			const pattern = /^STATUS:\s*(.+)$/;
			var status_line = results[0]
			// console.info("status line: " + status_line);
			const match = pattern.exec(status_line)
			if( match ) {
			var status_tmp = match[1]
			if (match[1].toLowerCase() == "ok" ) {
				status = 1;
			} else {
				status = 0;
			}
			// } else {
			//  console.info("pattern not found")
			}
		}
		status
	`
	js, err := NewJSCode(code, nil)
	if err != nil {
		assert.Nil(t, err, fmt.Sprintf(`TestJSModuleAnalyzeComplex compilation error: %s`, err.Error()))
		return
	}

	symtab := make(map[string]any)
	results := []string{
		"STATUS: OK",
		"",
	}
	symtab["results"] = results

	v, err := js.Run(symtab, nil)
	if err != nil {
		assert.Nil(t, err, fmt.Sprintf(`TestJSModuleAnalyzeComplex execution error: %s`, err.Error()))
		return
	}
	if value, ok := v.(int64); !ok {
		assert.True(t, ok, fmt.Sprintf("TestJSModuleAnalyzeComplex exec return value type differs: %v", v))
	} else {
		assert.True(t, value == 1, fmt.Sprintf("TestJSModuleAnalyzeComplex exec return invalid value: %v", value))
	}

}

func TestJSModuleConsole(t *testing.T) {
	code := `
	console.log('js: console.log(a)')
	console.error('js: console.error(b)')
	console.warn('js: console.warn(c)')
	console.info('js: console.info(d)')
	console.debug('js: console.debug(e)')
	"ok"
`
	logHandlerOpts := &slog.HandlerOptions{
		Level:     slog.LevelDebug,
		AddSource: true,
	}
	logger := slog.New(slog.NewJSONHandler(os.Stderr, logHandlerOpts))

	js, err := NewJSCode(code, nil)
	if err != nil {
		assert.Nil(t, err, fmt.Sprintf(`TestJSModuleConsole compilation error: %s`, err.Error()))
		return
	}
	//	logger.Info("a")
	v, err := js.Run(nil, logger)
	if err != nil {
		assert.Nil(t, err, fmt.Sprintf(`TestJSModuleConsole compilation error: %s`, err.Error()))
		return
	}
	if value, ok := v.(string); !ok {
		assert.True(t, ok, fmt.Sprintf("TestJSModuleConsole exec return value type differs: %v", v))
	} else {
		assert.True(t, value == "ok", fmt.Sprintf("TestJSModuleConsole exec return invalid value: %s", value))
	}
}

func TestJSModuleExporterConvertToBytes(t *testing.T) {
	code := `exporter.convertToBytes( 13.45, "Mb" )`

	js, err := NewJSCode(code, template.Js_func_map())
	if err != nil {
		assert.Nil(t, err, fmt.Sprintf(`TestJSModuleExporterConvertToBytes compilation error: %s`, err.Error()))
		return
	}

	v, err := js.Run(nil, nil)
	if err != nil {
		assert.Nil(t, err, fmt.Sprintf(`TestJSModuleExporterConvertToBytes compilation error: %s`, err.Error()))
		return
	}
	if value, ok := v.(int64); !ok {
		assert.True(t, ok, fmt.Sprintf("TestJSModuleExporterConvertToBytes exec return value type differs) %v", v))
	} else {
		assert.True(t, value == 13631488, fmt.Sprintf("TestJSModuleExporterConvertToBytes exec return invalid value) %d", value))
	}
}

func TestJSModuleExporterDefault(t *testing.T) {
	code := `
		"node:" + exporter.default(item.node, "undef") +
		 "-port:" + exporter.default(item.port, "undef") 
		`
	symtab := make(map[string]any)
	item := make(map[string]any)
	item["node"] = "node1"
	symtab["item"] = item

	js, err := NewJSCode(code, template.Js_func_map())
	if err != nil {
		assert.Nil(t, err, fmt.Sprintf(`TestJSModuleExporterDefault compilation error: %s`, err.Error()))
		return
	}

	v, err := js.Run(symtab, nil)
	if err != nil {
		assert.Nil(t, err, fmt.Sprintf(`TestJSModuleExporterDefault compilation error: %s`, err.Error()))
		return
	}
	if value, ok := v.(string); !ok {
		assert.True(t, ok, fmt.Sprintf("TestJSModuleExporterDefault exec return value type differs) %v", v))
	} else {
		tab := strings.Split(value, "-")
		if len(tab) == 2 {
			assert.True(t, strings.Contains(tab[0], "node1"), fmt.Sprintf("TestJSModuleExporterDefault exec return invalid value for node %s", tab[0]))
			assert.True(t, strings.Contains(tab[1], "undef"), fmt.Sprintf("TestJSModuleExporterDefault exec return invalid value for port %s", tab[1]))
		} else {
			assert.True(t, false, fmt.Sprintf("TestJSModuleExporterDefault exec invalid (not '-' separator) return %s", value))
		}
	}

}

func TestJSModuleExporterDecryptPass(t *testing.T) {
	// code := `
	// 	var res = ''
	// 	try {
	// 		res = exporter.decryptPass( "/encrypted/empty", "012345" )
	// 	}
	// 	catch (err) {
	// 		res = "error: " + err
	// 	}
	// 	res
	// `
	symtab := make(map[string]any)

	// test 1
	code := `exporter.decryptPass( password, shared_key )`
	js, err := NewJSCode(code, template.Js_func_map())
	if err != nil {
		assert.Nil(t, err, fmt.Sprintf(`TestJSModuleExporterDecryptPass compilation error: %s`, err.Error()))
		return
	}

	symtab["password"] = "/encrypted/empty"
	symtab["shared_key"] = "012345"

	_, err = js.Run(symtab, nil)
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "can't obtain cipher to decrypt"))

	// test 2
	symtab["shared_key"] = "0123456789abcdef"
	_, err = js.Run(symtab, nil)
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "invalid key provided to decrypt"))

	// test 3
	symtab["password"] = "/encrypted/CsG1r/o52tjX6zZH+uHHbQx97BaHTnayaGNP0tcTHLGpt5lMesw="
	res, err := js.Run(symtab, nil)
	assert.Nil(t, err)
	if passwd, ok := res.(string); !ok {
		assert.True(t, ok, "invalid type for result ")
	} else {
		assert.True(t, passwd == "mypassword")
	}

}

func TestJSModuleExporterGetDurationSecond(t *testing.T) {
	symtab := make(map[string]any)

	// test 1
	code := `
	Math.floor(new Date('2025-06-05T07:20:00+0000').getTime()/1000) + exporter.getDurationSecond( configCacheDuration )
	`

	js, err := NewJSCode(code, template.Js_func_map())
	if err != nil {
		assert.Nil(t, err, fmt.Sprintf(`TestJSModuleExporterGetDurationSecond compilation error: %s`, err.Error()))
		return
	}

	symtab["configCacheDuration"] = "1"

	_, err = js.Run(symtab, nil)
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "can't extract duration"), "invalid error:", err.Error())

	// test 2

	symtab["configCacheDuration"] = "1h"

	res, err := js.Run(symtab, nil)
	assert.Nil(t, err)
	if ts, ok := res.(int64); !ok {
		assert.True(t, ok, "invalid type for result ")
	} else {
		assert.True(t, ts == 1749111600, "incorrect value obtained", ts)
	}

	// test 3
	symtab["lastDate"] = "2025-05-14 16:24"

	code = `
		Math.floor(new Date( lastDate ).getTime()/1000) + exporter.getDurationSecond( configCacheDuration )
	`

	js, err = NewJSCode(code, template.Js_func_map())
	if err != nil {
		assert.Nil(t, err, fmt.Sprintf(`TestJSModuleExporterGetDurationSecond compilation error: %s`, err.Error()))
		return
	}

	symtab["configCacheDuration"] = "1"

	_, err = js.Run(symtab, nil)
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "TestJSModuleExporterGetDurationSecond: can't extract duration"), "invalid error:", err.Error())

}
