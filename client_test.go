package main

import (
	"encoding/json"
	"encoding/xml"
	"log/slog"
	"net/http"
	"os"
	"reflect"
	"testing"

	"github.com/go-resty/resty/v2"
)

func TestParseJsonResponse(t *testing.T) {
	client := &Client{
		client: resty.New(),
		symtab: map[string]any{},
		logger: &slog.Logger{},
	}
	file, err := os.ReadFile("fixtures/response.json")
	if err != nil {
		t.Errorf(`ParseJsonResponse() load test result error: %s`, err.Error())
		return
	}
	var data any
	if err := json.Unmarshal(file, &data); err != nil {
		t.Errorf(`ParseJsonResponse() parsing test result error: %s`, err.Error())
		return
	}
	raw_http := &http.Response{
		Header: make(map[string][]string),
	}
	raw_http.Header.Add(contentTypeHeader, "application/json")
	resp := &resty.Response{
		RawResponse: raw_http,
	}
	resp.SetBody(file)
	res_data := client.getResponse(resp, "json")
	if !reflect.DeepEqual(data, res_data) {
		t.Errorf(`ParseJsonResponse() parsed json result differ`)
	}
}

func TestParseXMLResponse(t *testing.T) {
	client := &Client{
		client: resty.New(),
		symtab: map[string]any{},
		logger: &slog.Logger{},
	}
	file, err := os.ReadFile("fixtures/response.xml")
	if err != nil {
		t.Errorf(`ParseXMLResponse() load test result error: %s`, err.Error())
		return
	}
	var (
		data_internal *Content
		data          any
	)
	if err := xml.Unmarshal(file, &data_internal); err != nil {
		t.Errorf(`ParseXMLResponse() parsing test results error: %s`, err.Error())
		return
	}
	data_tmp := make(map[string]any)
	data_tmp[data_internal.Name] = data_internal.Attrs
	data = data_tmp

	raw_http := &http.Response{
		Header: make(map[string][]string),
	}
	raw_http.Header.Add(contentTypeHeader, "text/xml")
	resp := &resty.Response{
		RawResponse: raw_http,
	}
	resp.SetBody(file)
	res_data := client.getResponse(resp, "xml")
	if !reflect.DeepEqual(data, res_data) {
		t.Errorf(`ParseXMLResponse() parsed xml results differ`)
	}
}
