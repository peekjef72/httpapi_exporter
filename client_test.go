package main

import (
	"encoding/json"
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
	if reflect.DeepEqual(data, res_data) {
		t.Errorf(`ParseJsonResponse() parsing json result differ`)
	}
}
