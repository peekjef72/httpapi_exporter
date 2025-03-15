package main

import (
	"fmt"
	"strings"
	"testing"

	ttemplate "text/template"

	"github.com/stretchr/testify/assert"
)

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
	tmpl := ttemplate.New("test").Funcs(mymap())

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
