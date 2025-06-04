package template

import (
	"fmt"
	"strings"
	"testing"

	ttemplate "text/template"

	"github.com/stretchr/testify/assert"
)

func TestFuncExporterSet(t *testing.T) {
	symtab := make(map[string]any)

	symtab["test"] = 1
	_, err := exporterSet(symtab, "test2", 2)
	assert.Nil(t, err)
	if raw_v, ok := symtab["test2"]; ok {
		if val, ok := raw_v.(int); ok {
			assert.True(t, val == 2, "invalid value stored")
		} else {
			assert.True(t, ok, "test key has invalid type")
		}
	} else {
		assert.True(t, ok, "test2 key not found in map")
	}

	my_map := make(map[string]any)
	my_map["key1"] = "value1"
	sub_list := make([]string, 2)
	sub_list[0] = "sub_value1"
	sub_list[1] = "sub_value2"
	my_map["sub_list_1"] = sub_list

	_, err = exporterSet(symtab, "my_map", my_map)
	assert.Nil(t, err)
	if raw_v, ok := symtab["my_map"]; ok {
		if _, ok := raw_v.(map[string]any); ok {
			assert.True(t, ok, "test key has invalid type")
		}
	} else {
		assert.True(t, ok, "my_map key not found in map")
	}

	template_str := `{{ $info := dict "user" "myuser" "password" "mypassword" }}
				{{ $tmp := exporterSet .data "info" $info }}
                {{ .data | toRawJson }}`

	tmpl := ttemplate.New("test").Funcs(Mymap())

	tmpl, err = tmpl.Parse(template_str)
	assert.Nil(t, err, fmt.Errorf("test template %s is invalid: %s", template_str, err))
	if tmpl != nil {
		tmp_res := new(strings.Builder)
		item := make(map[string]any)

		item["data"] = make(map[string]any)
		err = ((*ttemplate.Template)(tmpl)).Execute(tmp_res, &item)
		assert.Nil(t, err)

		// obtain final string from builder
		if err == nil {
			tmp := strings.TrimSpace(tmp_res.String())
			assert.True(t, tmp == `{"info":{"password":"mypassword","user":"myuser"}}`, tmp)
		}
	}
}

// func TestFuncExporterKeys(t *testing.T) {
// 	symtab := make(map[string]any)
// }

// func TestFuncExporterValues(t *testing.T) {
// 	symtab := make(map[string]any)
// }

func TestFuncExporterDecryptPass(t *testing.T) {
	passwd := "/encrypted/empty"
	res, err := exporterDecryptPass(passwd, "012345")
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "can't obtain cipher to decrypt"))
	assert.True(t, res == passwd)

	res, err = exporterDecryptPass(passwd, "0123456789abcdef")
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "invalid key provided to decrypt"))
	assert.True(t, res == "")

	passwd = "/encrypted/CsG1r/o52tjX6zZH+uHHbQx97BaHTnayaGNP0tcTHLGpt5lMesw="
	res, err = exporterDecryptPass(passwd, "0123456789abcdef")
	assert.Nil(t, err)
	assert.True(t, res == "mypassword")

	template_str := `{{ $tmp_pass := exporterDecryptPass .password .auth_key }}
                  {{ $data := dict "password" $tmp_pass }}
                  {{ $data | toRawJson }}`
	tmpl := ttemplate.New("test").Funcs(Mymap())

	tmpl, err = tmpl.Parse(template_str)
	assert.Nil(t, err, fmt.Errorf("test template %s is invalid: %s", template_str, err))
	if tmpl != nil {
		tmp_res := new(strings.Builder)
		item := make(map[string]string)

		item["auth_key"] = "0123456789abcdef"
		item["password"] = "/encrypted/CsG1r/o52tjX6zZH+uHHbQx97BaHTnayaGNP0tcTHLGpt5lMesw="
		err = ((*ttemplate.Template)(tmpl)).Execute(tmp_res, &item)
		assert.Nil(t, err)

		// obtain final string from builder
		if err == nil {
			tmp := strings.TrimSpace(tmp_res.String())
			assert.True(t, tmp == `{"password":"mypassword"}`)
		}
	}
}

func TestFuncExportLookupAddr(t *testing.T) {
	res, err := exportLookupAddr("don_t_find")
	assert.Nil(t, err)
	assert.True(t, res == "<no reverse host>")

	res, _ = exportLookupAddr("127.0.0.1")
	assert.Nil(t, err)
	assert.True(t, res == "localhost")

	res, _ = exportLookupAddr("::1")
	assert.Nil(t, err)
	assert.True(t, res == "localhost")

}

func TestFuncExporterRegexExtract(t *testing.T) {

	res, err := exporterRegexExtract("bla[.*", "status:OK")
	assert.NotNil(t, err)
	assert.Nil(t, res)

	regex := `^status:\s*(.*)$`

	res, err = exporterRegexExtract(regex, "status:OK")
	assert.Nil(t, err)
	assert.True(t, len(res) > 0)
	if len(res) > 0 {
		assert.True(t, res[1] == "OK")
	}

	regex = `^StAtus:\s*(.*)$`
	res, err = exporterRegexExtract(regex, "status:OK")
	assert.Nil(t, err)
	assert.True(t, len(res) == 0)

}

func TestTemplateExporterRegexExtract(t *testing.T) {
	var err error
	template_str := `{{ index ( exporterRegexExtract "^status:\\s*(.*)$" .res) 1 }}`
	tmpl := ttemplate.New("test").Funcs(Mymap())

	tmpl, err = tmpl.Parse(template_str)
	assert.Nil(t, err, fmt.Errorf("test template %s is invalid: %s", template_str, err))
	if tmpl != nil {
		tmp_res := new(strings.Builder)
		item := make(map[string]string)

		item["res"] = "status:OK"
		err = ((*ttemplate.Template)(tmpl)).Execute(tmp_res, &item)
		assert.Nil(t, err)

		// obtain final string from builder
		if err == nil {
			tmp := tmp_res.String()
			assert.True(t, tmp == "OK")
		}

		item["res"] = "status: invalid"
		tmp_res.Reset()
		err = ((*ttemplate.Template)(tmpl)).Execute(tmp_res, &item)
		assert.Nil(t, err)

		// obtain final string from builder
		if err == nil {
			tmp := tmp_res.String()
			assert.True(t, tmp == "invalid")
		}

		item["res"] = "not matching line"
		tmp_res.Reset()
		err = ((*ttemplate.Template)(tmpl)).Execute(tmp_res, &item)
		assert.NotNil(t, err)

	}
}
