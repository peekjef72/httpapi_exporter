// cSpell:ignore stretchr, yamlscript, regrTest, cerebro, lastdate, devcounter

package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func TestMetricsGoTemplate(t *testing.T) {

	// init pre-requirements for yamlscript to work
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
	// add constants to symbols table so that script can work
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

func TestMetricsJSTemplate(t *testing.T) {

	// init pre requirements for yamlscript to work
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
            position: >
                js: "cage-" + exporter.default( position.cage, "undef") +
                    "/Port-" + exporter.default( position.slot, "undef" ) + 
                    "/diskPos-" + exporter.default(position.diskPos, "undef" )
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
	symtab["__collector_id"] = "regrTest"
	symtab["__name__"] = "TestMetricsScript"
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
	var_name := `js: results.members[idx]`
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

func TestMetricsRegressionPSF(t *testing.T) {

	// init pre requirements for yamlscript to work
	initTest()

	logHandlerOpts := &slog.HandlerOptions{
		Level:     slog.LevelDebug,
		AddSource: true,
	}
	logger = slog.New(slog.NewJSONHandler(os.Stderr, logHandlerOpts))

	// the script part to execute.
	code := `
    - name: proceed elements
      loop: $results
      when: 'js: item.uniqueId != undefined'
      metrics:
        - metric_name: availability_percent
          help: component availability in percent.
          type: gauge
          key_labels:
            solution: _
            uniqueid: $uniqueId
          values:
            _: $availability

        - metric_name: availability_timestamp
          help: timestamp when the component availability was computed.
          type: gauge
          key_labels:
            solution: _
            uniqueid: $uniqueId
          values:
            _: 'js: Math.floor( new Date( lastDate + "+00:00").getTime() / 1000 )'

    - name: proceed perf query
      scope: none
      metrics:
        - metric_name: query_perf_seconds
          help: "query stage duration in seconds"
          type: gauge
          key_labels:
            stage: $stage
            page: /psf-ms-cerebro/status
          values:
            _: $trace_infos.${stage}
          with_items: 'js: Object.keys( trace_infos )'
          loop_var: stage
`

	script := &YAMLScript{
		name: "test",
	}
	// parse the code and build AST to execute.
	err := yaml.Unmarshal([]byte(code), &script)
	if err != nil {
		assert.Nil(t, err, fmt.Sprintf(`TestMetricsRegressionPSF("%s") error: %s`, script.name, err.Error()))
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
	symtab["__collector_id"] = "regrTest"
	symtab["__name__"] = "TestMetricsRegressionPSF"
	symtab["query_status"] = true

	// set the data to build metrics content
	results_str := `[ {
		"uniqueId": "149d2aca-43bf-45d2-986a-0d268cd79295",
		"solution": "ALF",
		"lastDate": "2025-05-08 10:29",
		"availability": 100
	}, {
		"uniqueId": "cee03dda-fa0c-4a5f-90aa-1d3fc158ed1e",
		"solution": "OAB",
		"lastDate": "2025-05-08 10:29",
		"availability": 100
	} ]
	`

	var data any
	if err := json.Unmarshal([]byte(results_str), &data); err != nil {
		t.Errorf(`TestMetricsRegressionPSF() parsing test results error: %s`, err.Error())
		return
	}
	symtab["results"] = data

	trace_infos := make(map[string]any)
	trace_infos["conn_time"] = 8.700078479
	trace_infos["dns_lookup"] = 4.7756e-05
	trace_infos["response_time"] = 8.4431e-05
	trace_infos["server_time"] = 1.822854087
	trace_infos["tcp_con_time"] = 4.13496939
	trace_infos["tls_handshake"] = 4.564948476
	trace_infos["total_time"] = 10.52295534
	symtab["trace_infos"] = trace_infos

	// set the channel to send results so that we can receive & analyze them
	metricChan := make(chan Metric, capMetricChan)
	symtab["__metric_channel"] = (chan<- Metric)(metricChan)

	// play the script
	err = script.Play(symtab, false, logger)
	if err != nil {
		assert.Nil(t, err, `TestMetricsRegressionPSF("%s") error: %s`, script.name, err.Error())
		return
	}

	logger.Debug(fmt.Sprintf("metric channel length: %d", len(metricChan)))

	// build a var to obtain member to compare metrics and initial data
	var_name := `js: results[idx]`
	name, err := NewField(var_name, nil)
	if err != nil {
		t.Errorf(`TestMetricsRegressionPSF("%s") error: %s`, var_name, err.Error())
		return
	}

	for idx := range len(metricChan) {
		var (
			res map[string]any
		)
		if idx < 4 {
			// set idx value so that var name point to correct res
			symtab["idx"] = int(idx / 2)
			if r_value, err := ValorizeValue(symtab, name, logger, "TestMetricsScript", false); err != nil {
				t.Errorf(`TestMetricsScript("%s") error: %s`, var_name, err.Error())
			} else {
				if val, ok := r_value.(map[string]any); !ok {
					t.Errorf(`TestMetricsScript("%s") not []string ! %v`, var_name, r_value)
					return
				} else {
					res = val
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
				switch idx {
				case 0, 2:
					assert.True(t, metricDesc.Name() == "availability_percent", "invalid name found", metricDesc.Name())

					// check value
					if f_value, ok := GetMapValueFloat(res, "availability"); !ok {
						assert.True(t, ok, fmt.Sprintf("TestMetricsScript('%s') value not found", var_name))
					} else {
						if metric.val != f_value {
							logger.Debug("metric_val", "m.Value", metric.val, "awaiting value", f_value)
						}
						assert.True(t, metric.val == f_value, "metric value differs between computed and source", metricDesc.Name(), metric.val, f_value)
					}
				case 1, 3:
					assert.True(t, metricDesc.Name() == "availability_timestamp", "invalid name found", metricDesc.Name())

					// check value
					lastdate := GetMapValueString(res, "lastDate")
					if lastdate == "" {
						assert.True(t, lastdate == "", fmt.Sprintf("TestMetricsScript('%s') value not found", var_name))
					} else {
						if ts, err := time.Parse("2006-01-02 15:04:00 -0700", lastdate+":00 +0000"); err != nil {
							assert.Nil(t, err, `TestMetricsRegressionPSF("%s") error: %s`, script.name, err.Error())
							return
						} else {
							f_value := float64(ts.Unix())
							assert.True(t, metric.val == f_value,
								fmt.Sprintf("metric value differs between computed and source=> metric: %s - obtained: %f - set %f", metricDesc.Name(), f_value, metric.val))
						}
					}
				}
				// check labels
				trans_map := make(map[string]string)
				trans_map["solution"] = "solution"
				trans_map["uniqueid"] = "uniqueId"

				for _, label := range metric.labelPairs {

					if _, ok := trans_map[*label.Name]; !ok {
						continue
					}
					if value := GetMapValueString(res, trans_map[*label.Name]); value == "" {
						assert.True(t, ok, fmt.Sprintf("TestMetricsScript('%s') label val source  not found", name))
					} else {
						assert.True(t, *label.Value == value,
							"label value differs between computed and source",
							*label.Value, value)
					}
				}
			}
		} else {
			break
		}
	}
	for range len(metricChan) {

		// collect current metric
		metric_raw := <-metricChan
		if metric, ok := metric_raw.(invalidMetric); ok {
			assert.Nil(t, metric.err)
			return
		} else if metric, ok := metric_raw.(*constMetric); ok {
			// check metric name
			metricDesc := metric.Desc()
			assert.True(t, metricDesc.Name() == "query_perf_seconds", "invalid name found", metricDesc.Name())
			stage := ""
			for _, label := range metric.labelPairs {
				if *label.Name == "stage" {
					stage = *label.Value
					break
				}
			}
			// check value
			if f_value, ok := GetMapValueFloat(trace_infos, stage); !ok {
				assert.True(t, ok, fmt.Sprintf("TestMetricsScript('%s') value not found", stage))
			} else {
				assert.True(t, metric.val == f_value,
					fmt.Sprintf("metric value differs between computed and source: stage:%s obtained=%f set=%f", stage, metric.val, f_value))
			}

		}

	}
}

// DEBUG prog
func TestMetricsJSTempo(t *testing.T) {

	// init pre-requirements for yamlscript to work
	initTest()

	logHandlerOpts := &slog.HandlerOptions{
		Level:     slog.LevelDebug,
		AddSource: true,
	}
	logger = slog.New(slog.NewJSONHandler(os.Stderr, logHandlerOpts))

	// the script part to execute.
	code := `
    - name: extract device
      set_fact:
        dev1: >-
          js:
            var res = devcounter.actions.split('/')
            res[4]

    - name: loop on metrics
      loop: $devcounter.metrics
      loop_var: metric
      actions:
        - name: loop on bins
          loop: $metric.bins
          loop_var: bin
          actions:
            - name: debug
              debug:
                msg: "bin: {{ .bin }}"

            - name: build
              metrics:
                - metric_name: action_metrics_total
                  scope: none
                  type: gauge
                  help: "dummy & dummy"
                  key_labels:
                    device: $dev1
                    partId: $metric.partId
                    label: $metric.label
                    bindID: $bin.binId
                    counterID: $bin.counters[0].counterId
                  values:
                    _: $bin.counters[0].count
    - name: add metric for reset resetTime
      metrics:
        - metric_name: action_metrics_reset_time
          # scope: none
          type: gauge
          help: reset timestamp
          keys_labels:
            device: $dev1
          values:
            _: >-
              js:
                 Math.floor(new Date( devcounter.resetTime +'+0000').getTime()/1000)
`

	script := &YAMLScript{
		name: "test",
	}
	// parse the code and build AST to execute.
	err := yaml.Unmarshal([]byte(code), &script)
	if err != nil {
		assert.Nil(t, err, fmt.Sprintf(`TestMetricsJSTempo("%s") error: %s`, script.name, err.Error()))
		return
	}

	// set metric associated with found code.
	var logContext []any
	for _, ma := range script.metricsActions {
		for _, act := range ma.Actions {
			if act.Type() == metric_action {
				mc := act.GetMetric()
				if mc == nil {
					assert.Nil(t, mc, fmt.Errorf(`TestMetricsJSTempo("%s") MetricAction nil received`, script.name))
					return
				}
				mf, err := NewMetricFamily(logContext, mc, nil, nil)
				if err != nil {
					assert.Nil(t, err, fmt.Errorf(`TestMetricsJSTempo("%s") MetricAction nil received`, script.name))
					return
				}
				//			ma.metricFamilies = append(ma.metricFamilies, mf)
				// mfs = append(mfs, mf)
				act.SetMetricFamily(mf)
			}
		}
	}
	// add constants to symbols table so that script wan work
	symtab["__collector_id"] = "test"
	symtab["__name__"] = "TestMetricsJSTempo"
	symtab["query_status"] = true

	// set the data to build metrics content
	file_content, err := os.ReadFile("fixtures/temp_data.json")
	if err != nil {
		t.Errorf(`TestMetricsJSTempo("%s") load test results error: %s`, script.name, err.Error())
		return
	}
	var data any
	if err := json.Unmarshal(file_content, &data); err != nil {
		t.Errorf(`TestMetricsJSTempo("%s") parsing test results error: %s`, script.name, err.Error())
		return
	}
	symtab["devcounter"] = data

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

}
