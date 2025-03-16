package main

import (
	"encoding/json"
	"encoding/xml"
	"log/slog"
	"net/http"
	"os"
	"reflect"
	"regexp"
	"testing"

	"github.com/go-resty/resty/v2"
	"gopkg.in/yaml.v3"
)

func TestParseJsonResponse(t *testing.T) {
	client := &Client{
		client: resty.New(),
		symtab: map[string]any{},
		logger: &slog.Logger{},
	}
	file_content, err := os.ReadFile("fixtures/response.json")
	if err != nil {
		t.Errorf(`ParseJsonResponse() load test results error: %s`, err.Error())
		return
	}
	var data any
	if err := json.Unmarshal(file_content, &data); err != nil {
		t.Errorf(`ParseJsonResponse() parsing test results error: %s`, err.Error())
		return
	}
	raw_http := &http.Response{
		Header: make(map[string][]string),
	}
	raw_http.Header.Add(contentTypeHeader, "application/json")
	resp := &resty.Response{
		RawResponse: raw_http,
	}
	resp.SetBody(file_content)
	res_data := client.getResponse(resp, "json")
	if !reflect.DeepEqual(data, res_data) {
		t.Errorf(`ParseJsonResponse() parsed json results differ`)
	}
}

func TestParseXMLResponse(t *testing.T) {
	client := &Client{
		client: resty.New(),
		symtab: map[string]any{},
		logger: &slog.Logger{},
	}
	file_content, err := os.ReadFile("fixtures/response.xml")
	if err != nil {
		t.Errorf(`ParseXMLResponse() load test result error: %s`, err.Error())
		return
	}
	var (
		data_internal *Content
		data          any
	)
	if err := xml.Unmarshal(file_content, &data_internal); err != nil {
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
	resp.SetBody(file_content)
	res_data := client.getResponse(resp, "xml")
	if !reflect.DeepEqual(data, res_data) {
		t.Errorf(`ParseXMLResponse() parsed xml results differ`)
	}
}

func TestParseYAMLResponse(t *testing.T) {
	client := &Client{
		client: resty.New(),
		symtab: map[string]any{},
		logger: &slog.Logger{},
	}
	file_content, err := os.ReadFile("fixtures/response_simple.yml")
	if err != nil {
		t.Errorf(`ParseYAMLResponse() load test results error: %s`, err.Error())
		return
	}
	var (
		data_internal any
	)
	if err := yaml.Unmarshal(file_content, &data_internal); err != nil {
		t.Errorf(`ParseYAMLResponse() parsing test results error: %s`, err.Error())
		return
	}
	raw_http := &http.Response{
		Header: make(map[string][]string),
	}
	raw_http.Header.Add(contentTypeHeader, "application/yaml")
	resp := &resty.Response{
		RawResponse: raw_http,
	}
	resp.SetBody(file_content)
	res_data := client.getResponse(resp, "yaml")
	if !reflect.DeepEqual(data_internal, res_data) {
		t.Errorf(`ParseYAMLResponse() parsed json results differ`)
	}

}

func TestParseTextLineResponse(t *testing.T) {
	client := &Client{
		client: resty.New(),
		symtab: map[string]any{},
		logger: &slog.Logger{},
	}
	file_content, err := os.ReadFile("fixtures/response_line_simple.txt")
	if err != nil {
		t.Errorf(`ParseTextLineResponse() load test results error: %s`, err.Error())
		return
	}
	var data []string

	if re, err := regexp.Compile("\r?\n"); err != nil {
		t.Errorf(`ParseTextLineResponse() parsing test results error: %s`, err.Error())
		return
	} else {
		data = re.Split(string(file_content), -1)
	}

	raw_http := &http.Response{
		Header: make(map[string][]string),
	}
	raw_http.Header.Add(contentTypeHeader, "text/plain")
	resp := &resty.Response{
		RawResponse: raw_http,
	}
	resp.SetBody(file_content)
	res_data := client.getResponse(resp, "text-lines")
	if !reflect.DeepEqual(data, res_data) {
		t.Errorf(`ParseTextLineResponse() parsed text results differ`)
	}

}
