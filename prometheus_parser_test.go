package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFuncParsePrometheusResponse(t *testing.T) {
	file_content, err := os.ReadFile("fixtures/response.prom")
	if err != nil {
		t.Errorf(`ParsePrometheusResponse() load test results error: %s`, err.Error())
		return
	}
	var (
		raw_data any
	)
	if data, err := ParsePrometheusResponse(file_content); err != nil {
		t.Errorf(`ParseYAMLResponse() parsing test results error: %s`, err.Error())
		return
	} else {
		raw_data = data
	}
	if data, ok := raw_data.(map[string]any); !ok {
		t.Errorf("ParsePrometheusResponse(): invalid content received")
	} else {
		// must obtain 9 metrics
		assert.True(t, len(data) == 9, "result doesn't contain 9 metrics")

		// check that metric familly  "apache_workers" exists and contains two metrics
		if raw_metric, ok := data["apache_workers"]; !ok {
			t.Errorf(`ParsePrometheusResponse(): metric "apache_workers" not founf`)
		} else {
			if metric, ok := raw_metric.(map[string]any); !ok {
				t.Errorf("ParsePrometheusResponse(): invalid content for metric")
			} else {
				met_type := GetMapValueString(metric, "type")
				assert.True(t, met_type == "gauge", "ParsePrometheusResponse(): invalid metric type found")
				raw_data := metric["metrics"]
				if mets, ok := raw_data.([]map[string]any); !ok {
					t.Errorf(`ParsePrometheusResponse(): invalid content for metric["metrics"]`)
				} else {
					assert.True(t, len(mets) == 2, "ParsePrometheusResponse(): invalid metric type found")
				}
			}

		}
	}
}
