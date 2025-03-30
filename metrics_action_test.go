package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func TestMetrics(t *testing.T) {

	// init prerequirements for yamlscript to work
	initTest()

	logHandlerOpts := &slog.HandlerOptions{
		Level:     slog.LevelDebug,
		AddSource: true,
	}
	logger = slog.New(slog.NewJSONHandler(os.Stderr, logHandlerOpts))

	// the script part to execute.
	code := `
    - name: collect disks
      scope: results
      metrics:
        - metric_name: disk_status
          help: "physical disk status: 0: normal - 1: degraded - 2: New - 4: Failed - 99: Unknown"
          type: gauge
          key_labels:
            model: _
            serial: $serialNumber
            position: cage-{{ .position.cage | default "undef" }}/Port-{{ .position.slot | default "undef" }}/diskPos-{{ .position.diskPos | default "undef" }}
            capacity: $mfgCapacityGB
          values:
            _ : $state
          loop: $members
`

	script := &YAMLScript{
		name: "test",
	}
	// parse the code and build AST to execute.
	err := yaml.Unmarshal([]byte(code), &script)
	if err != nil {
		assert.Nil(t, err, fmt.Sprintf(`TestMetricsScript("%s") error: %s`, script.name, err.Error()))
		return
	}

	// set metric associated with found code.
	var logContext []any
	for _, ma := range script.metricsActions {
		for _, act := range ma.Actions {
			if act.Type() == metric_action {
				mc := act.GetMetric()
				if mc == nil {
					assert.Nil(t, mc, fmt.Errorf("MetricAction nil received"))
					return
				}
				mf, err := NewMetricFamily(logContext, mc, nil, nil)
				if err != nil {
					assert.Nil(t, err, fmt.Errorf("MetricAction nil received"))
					return
				}
				//			ma.metricFamilies = append(ma.metricFamilies, mf)
				// mfs = append(mfs, mf)
				act.SetMetricFamily(mf)
			}
		}
	}
	// add constants to symbols table so that script wan work
	symtab["__collector_id"] = "metrics_action_test.go"
	symtab["__name__"] = "TestMetrics"
	symtab["query_status"] = true

	// set the data to build metrics content
	results_str := `{
		"members":[
			{
				"state":1,
				"model": "model_value",
				"serialNumber": "0123456789",
				"mfgCapacityGB": 100,
				"position": {"cage":1, "slot": 1, "diskPos": 1}
			},{
				"state":1,
				"model":"model_value_2",
				"serialNumber": "1234567890",
				"mfgCapacityGB": 200,
				"position": {"cage":2, "slot": 3, "diskPos": 4}
			}
		]
	}`

	var data any
	if err := json.Unmarshal([]byte(results_str), &data); err != nil {
		t.Errorf(`TestMetricsScript() parsing test results error: %s`, err.Error())
		return
	}
	symtab["results"] = data

	// set the channel to send results so that we can receive & analyze them
	metricChan := make(chan Metric, capMetricChan)
	symtab["__metric_channel"] = (chan<- Metric)(metricChan)

	// play the script
	err = script.Play(symtab, false, logger)
	if err != nil {
		assert.Nil(t, err, `TestMetricsScript("%s") error: %s`, script.name, err.Error())
		return
	}

	logger.Debug(fmt.Sprintf("metric channel length: %d", len(metricChan)))

	// build a var to obtain member to compare metrics and initial data
	var_name := `$results.members[$idx]`
	name, err := NewField(var_name, nil)
	if err != nil {
		t.Errorf(`TestMetricsScript("%s") error: %s`, var_name, err.Error())
		return
	}

	for idx := range len(metricChan) {
		var (
			member map[string]any
		)

		// set idx value so that var name point to correct member
		symtab["idx"] = idx
		if r_value, err := ValorizeValue(symtab, name, logger, "TestMetricsScript", false); err != nil {
			t.Errorf(`TestMetricsScript("%s") error: %s`, var_name, err.Error())
		} else {
			if val, ok := r_value.(map[string]any); !ok {
				t.Errorf(`TestMetricsScript("%s") not []string ! %v`, var_name, r_value)
				return
			} else {
				member = val
			}
		}

		// collect current metric
		metric_raw := <-metricChan
		if metric, ok := metric_raw.(invalidMetric); ok {
			assert.Nil(t, metric.err)
			return
		} else if metric, ok := metric_raw.(*constMetric); ok {
			// check metric name
			metricDesc := metric.Desc()
			assert.True(t, metricDesc.Name() == "disk_status", "invalid name found", metricDesc.Name())

			// check value
			if f_value, ok := GetMapValueFloat(member, "state"); !ok {
				assert.True(t, ok, fmt.Sprintf("TestMetricsScript('%s') value not found", var_name))
			} else {
				assert.True(t, metric.val == f_value, "metric value differs between computed and source")
			}

			// check labels
			trans_map := make(map[string]string)
			trans_map["model"] = "model"
			trans_map["serial"] = "serialNumber"
			trans_map["capacity"] = "mfgCapacityGB"

			for _, label := range metric.labelPairs {

				if _, ok := trans_map[*label.Name]; !ok {
					continue
				}
				if value := GetMapValueString(member, trans_map[*label.Name]); value == "" {
					assert.True(t, ok, fmt.Sprintf("TestMetricsScript('%s') label val source  not found", name))
				} else {
					assert.True(t, *label.Value == value,
						"label value differs between computed and source",
						*label.Value, value)
				}
			}
		}
	}
}
