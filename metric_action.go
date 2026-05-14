// cSpell:ignore histo, ktype, htype

package main

import (
	//"bytes"
	"fmt"
	"log/slog"
	"reflect"
	"strings"

	"github.com/dop251/goja_nodejs/require"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/spf13/cast"
)

const (
	HistogramTypeUndef = iota
	HistogramTypeStatic
	HistogramTypeExternal

	metric_type_UNKNOWN = 255
)

type EHistogram struct {
	Type            int
	Histogram_var   *Field
	Histogram       []*prometheus.HistogramVec
	Buckets         *[]float64
	Histogram_value *Field
}

// MetricConfig defines a Prometheus metric, the SQL query to populate it and the mapping of columns to metric
// keys/values.
type MetricConfig struct {
	Name         string            `yaml:"metric_name" json:"metric_name"`                         // the Prometheus metric name
	TypeString   string            `yaml:"type" json:"type"`                                       // the Prometheus metric type
	Help         string            `yaml:"help" json:"help"`                                       // the Prometheus metric help text
	KeyLabels    any               `yaml:"key_labels,omitempty" json:"key_labels,omitempty"`       // expose these attributes as labels from JSON object: format name: value with name and value that should be template
	StaticLabels map[string]string `yaml:"static_labels,omitempty" json:"static_labels,omitempty"` // fixed key/value pairs as static labels
	ValueLabel   string            `yaml:"value_label,omitempty" json:"value_label,omitempty"`     // with multiple value columns, map their names under this label
	Values       map[string]string `yaml:"values" json:"values"`                                   // expose each of these columns as a value, keyed by column name
	Scope        string            `yaml:"scope,omitempty" json:"scope,omitempty"`                 // var path where to collect data: shortcut for {{ .scope.path.var }}

	HistogramInfos any `yaml:"histogram,omitempty" json:"histogram,omitempty"`

	// valueType_old prometheus.ValueType // TypeString converted to prometheus.ValueType
	valueType dto.MetricType

	registry       *require.Registry
	name           *Field
	help           *Field
	key_labels_map map[string]string
	key_labels     *Field
	prefix         string
	metric_type    *Field

	histogram *EHistogram
}

// ValueType returns the metric type, converted to a dto.MetricType.
func (m *MetricConfig) ValueType(symtab map[string]any, logger *slog.Logger) (dto.MetricType, error) {

	metric_type, err := m.metric_type.GetValueString(symtab, logger)
	if err != nil {
		return metric_type_UNKNOWN, err
	}

	switch strings.ToLower(metric_type) {
	case "counter":
		m.valueType = dto.MetricType_COUNTER
	case "gauge":
		m.valueType = dto.MetricType_GAUGE
	case "histogram":
		m.valueType = dto.MetricType_HISTOGRAM
	case "summary":
		m.valueType = dto.MetricType_SUMMARY
	default:
		return metric_type_UNKNOWN, fmt.Errorf("unsupported metric type: %s", m.TypeString)
	}
	return m.valueType, nil
}

// UnmarshalYAML implements the yaml.Unmarshaler interface for MetricConfig.
func (m *MetricConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type plain MetricConfig

	// set default type to gauge
	m.valueType = metric_type_UNKNOWN
	m.TypeString = "gauge"

	if err := unmarshal((*plain)(m)); err != nil {
		return err
	}
	// Check required fields
	if m.Name == "" {
		return fmt.Errorf("missing name for metric %+v", m)
	}
	if name, err := NewField(m.Name, nil, m.registry); err != nil {
		return err
	} else {
		m.name = name
	}

	if m.Help != "" {
		if help, err := NewField(m.Help, nil, m.registry); err == nil {
			m.help = help
		} else {
			return err
		}
	}

	if m.TypeString != "" {
		if type_str, err := NewField(m.TypeString, nil, m.registry); err == nil {
			m.metric_type = type_str
		} else {
			return err
		}
	}

	// Check for duplicate key labels
	if m.KeyLabels != nil {
		switch ktype := m.KeyLabels.(type) {
		case map[string]string:
			for key, val := range ktype {
				if err := checkLabel(key, "metric", m.Name); err != nil {
					return err
				}
				// specific for format key_name: _ => replace by ${key_name}
				if val == "_" {
					ktype[key] = "$" + key
				}
			}
			m.key_labels_map = ktype
		case map[string]any:
			m.key_labels_map = make(map[string]string, len(ktype))
			for key, val_raw := range ktype {
				if err := checkLabel(key, "metric", m.Name); err != nil {
					return err
				}
				if val, ok := val_raw.(string); ok {
					if val == "_" {
						val = "$" + key
					}
					m.key_labels_map[key] = val
				}
			}
		case string:
			if ktype != "" {
				if val, err := NewField(ktype, nil, m.registry); err == nil {
					m.key_labels = val
				} else {
					return err
				}
			}
		default:
			return fmt.Errorf("key_labels should be a map[string][string] or var(string) that will contain a map[string][string] for metric %q", m.Name)
		}
	}

	if m.HistogramInfos != nil {
		m.valueType = dto.MetricType_HISTOGRAM

		m.histogram = &EHistogram{}

		htype := reflect.ValueOf(m.HistogramInfos)
		switch htype.Kind() {
		case reflect.Map:
			// if we found elements definition (bucket & value) histogram is static
			m.histogram.Type = HistogramTypeStatic
			// hist := &EHistogram{
			// 	Type: HistogramTypeUndef,
			// }
			iter := htype.MapRange()
			for iter.Next() {
				raw_key := iter.Key()
				if raw_key.Kind() == reflect.String {
					switch raw_key.String() {
					case "buckets":
						t_buckets := iter.Value()
						invalid_format := false
						if t_buckets.Kind() == reflect.Interface {
							t_b_v := reflect.ValueOf(t_buckets.Interface())
							// t_b_i := reflect.TypeOf(t_buckets.Interface())
							if t_b_v.Kind() == reflect.Slice {
								buckets := make([]float64, t_b_v.Len())

								prev_v := 0.0
								for ind := range t_b_v.Len() {
									t_val := t_b_v.Index(ind)
									buckets[ind] = cast.ToFloat64(t_val.Interface())
									// check if bound of bucket is increasing strictly
									if ind > 0 {
										if buckets[ind] <= prev_v {
											return fmt.Errorf("invalid value for buckets bounds definition for histogram metric %q: element[%d] %f <= %f", m.Name, ind, buckets[ind], prev_v)
										}
										prev_v = buckets[ind]
									}
								}
								m.histogram.Buckets = &buckets
							} else {
								invalid_format = true
							}
						} else {
							invalid_format = true
						}
						if invalid_format {
							return fmt.Errorf("invalid format for buckets bounds definition for histogram metric %q: must be slice", m.Name)
						}

					case "value":
						t_value := iter.Value()
						invalid_format := false
						if t_value.Kind() == reflect.Interface {
							t_v_v := reflect.ValueOf(t_value.Interface())
							if t_v_v.Kind() == reflect.String {
								if val, err := NewField(t_v_v.String(), nil, m.registry); err == nil {
									m.histogram.Histogram_value = val
								} else {
									return err
								}
							} else {
								invalid_format = true
							}
						} else {
							invalid_format = true
						}
						if invalid_format {
							return fmt.Errorf("invalid format for value definition for histogram metric %q: must be string", m.Name)
						}
					}
				}
			}
			// need to check if both buckets and value are defined
			if m.histogram.Buckets == nil || len(*m.histogram.Buckets) == 0 || m.histogram.Histogram_value == nil {
				return fmt.Errorf("invalid definition for histogram metric %q: buckets and value must be set", m.Name)
				// } else {
				// 	toto := prometheus.NewHistogramVec(
				// 		prometheus.HistogramOpts {
				// 			Buckets: *m.histogram.Buckets,
				// 	}, []string{})
				// 	m.histogram.Histogram = &dto.Histogram{
				// 		Bucket: make([]*dto.Bucket, len(*m.histogram.Buckets)),
				// 	}
			}

		case reflect.String:
			m.histogram.Type = HistogramTypeExternal
			if htype.String() != "" {
				if val, err := NewField(htype.String(), nil, m.registry); err == nil {
					m.histogram.Histogram_var = val
				} else {
					return err
				}
			}
		default:
			return fmt.Errorf("histogram should be a map[string][string] or var(string) that will contain a map[string][string] for metric %q", m.Name)
		}
	} else if len(m.Values) == 0 {
		return fmt.Errorf("no values defined for metric %q", m.Name)
	}

	if len(m.Values) > 1 {
		// Multiple value columns but no value label to identify them
		if m.ValueLabel == "" {
			return fmt.Errorf("value_label must be defined for metric with multiple values %q", m.Name)
		}
		checkLabel(m.ValueLabel, "value_label for metric", m.Name)
	}

	return nil
}

// ***************************************************************************************
// ***************************************************************************************
// metric fact
// ***************************************************************************************
// ***************************************************************************************

type MetricAction struct {
	Name    *Field         `yaml:"name,omitempty" json:"name,omitempty"`
	With    []any          `yaml:"with,omitempty" json:"with,omitempty"`
	When    []*Field       `yaml:"when,omitempty" json:"when,omitempty"`
	LoopVar string         `yaml:"loop_var,omitempty" json:"loop_var,omitempty"`
	Vars    map[string]any `yaml:"vars,omitempty" json:"vars,omitempty"`
	Until   []*Field       `yaml:"until,omitempty" json:"until,omitempty"`

	mc           *MetricConfig
	metricFamily *MetricFamily
	vars         [][]any

	// Catches all undefined fields and must be empty after parsing.
	XXX map[string]interface{} `yaml:",inline" json:"-"`
}

func (a *MetricAction) Type() int {
	return metric_action
}
func (a *MetricAction) TypeName() string {
	return "metric_action"
}

func (a *MetricAction) GetName(symtab map[string]any, logger *slog.Logger) string {
	str, err := a.Name.GetValueString(symtab, logger)
	if err != nil {
		logger.Warn(
			fmt.Sprintf("invalid action name: %v", err),
			"coll", CollectorId(symtab, logger),
			"script", ScriptName(symtab, logger))
		return ""
	}
	return str
}

func (a *MetricAction) GetNameField() *Field {
	return a.Name
}
func (a *MetricAction) SetNameField(name *Field) {
	a.Name = name
}

func (a *MetricAction) GetWith() []any {
	return a.With
}
func (a *MetricAction) SetWith(with []any) {
	a.With = with
}

func (a *MetricAction) GetWhen() []*Field {
	return a.When

}
func (a *MetricAction) SetWhen(when []*Field) {
	a.When = when
}

func (a *MetricAction) GetLoopVar() string {
	return a.LoopVar
}
func (a *MetricAction) SetLoopVar(loopVar string) {
	a.LoopVar = loopVar
}

func (a *MetricAction) GetVars() [][]any {
	return a.vars
}
func (a *MetricAction) SetVars(vars [][]any) {
	a.vars = vars
}

func (a *MetricAction) GetUntil() []*Field {
	return a.Until
}
func (a *MetricAction) SetUntil(until []*Field) {
	a.Until = until
}

func (a *MetricAction) setBasicElement(
	registry *require.Registry,
	nameField *Field,
	vars [][]any,
	with []any,
	loopVar string,
	when []*Field,
	until []*Field) error {
	return setBasicElement(a, registry, nameField, vars, with, loopVar, when, until)
}

func (a *MetricAction) PlayAction(script *YAMLScript, symtab map[string]any, logger *slog.Logger) error {
	return PlayBaseAction(script, symtab, logger, a, a.CustomAction)
}

// only for MetricsAction
func (a *MetricAction) GetMetrics() []*GetMetricsRes {
	return nil
}

// only for MetricAction
func (a *MetricAction) GetMetric() *MetricConfig {
	return a.mc
}

func (a *MetricAction) SetMetricFamily(mf *MetricFamily) {
	a.metricFamily = mf
}

// only for PlayAction
func (a *MetricAction) SetPlayAction(scripts map[string]*YAMLScript) error {
	return nil
}

// specific behavior for the MetricAction

func (a *MetricAction) CustomAction(script *YAMLScript, symtab map[string]any, logger *slog.Logger) error {
	var (
		metric_channel chan<- Metric
		// mfs            []*MetricFamily
	)
	str_loop_var_idx := ""
	loop_var_idx := 0
	if val_loop_var_idx, ok := symtab["loop_var_idx"].(int); ok {
		if val_loop_var_idx > 0 {
			loop_var_idx = val_loop_var_idx
			str_loop_var_idx = fmt.Sprintf(" %d", loop_var_idx)
		}
	} else {
		str_loop_var_idx = "<no loop>"
	}
	logger.Debug(
		fmt.Sprintf("[Type: MetricAction] loop %s", str_loop_var_idx),
		"coll", CollectorId(symtab, logger),
		"script", ScriptName(symtab, logger),
		"name", a.GetName(symtab, logger))

	if r_val, ok := symtab["__metric_channel"]; ok {
		if metric_channel, ok = r_val.(chan<- Metric); !ok {
			panic(fmt.Sprintf("coll=\"%s\" script=\"%s\" name=\"%s\" msg=\"invalid context (channel wrong type)\"",
				CollectorId(symtab, logger),
				ScriptName(symtab, logger),
				a.GetName(symtab, logger)))
		}
	} else {
		panic(fmt.Sprintf("coll=\"%s\" script=\"%s\" name=\"%s\" msg=\"invalid context (channel not set)\"",
			CollectorId(symtab, logger),
			ScriptName(symtab, logger),
			a.GetName(symtab, logger)))
	}

	logger.Debug(
		fmt.Sprintf("    metric_name: %s", a.metricFamily.Name()),
		"coll", CollectorId(symtab, logger),
		"script", ScriptName(symtab, logger),
		"name", a.GetName(symtab, logger))

	metric_type := a.metricFamily.ValueType()

	if metric_type == dto.MetricType_HISTOGRAM {
		var histo *prometheus.HistogramVec
		if len(a.metricFamily.config.histogram.Histogram) > loop_var_idx {
			histo = a.metricFamily.config.histogram.Histogram[loop_var_idx]
		}
		symtab["__histogram"] = histo
	}
	a.metricFamily.Collect(symtab, logger, metric_channel)

	if metric_type == dto.MetricType_HISTOGRAM {
		if r_val, ok := symtab["__histogram"]; ok {
			if histo, ok := r_val.(*prometheus.HistogramVec); ok && histo != nil {
				if len(a.metricFamily.config.histogram.Histogram) > loop_var_idx {
					a.metricFamily.config.histogram.Histogram[loop_var_idx] = histo
				} else {
					a.metricFamily.config.histogram.Histogram = append(a.metricFamily.config.histogram.Histogram, histo)
				}
			}
			delete(symtab, "__histogram")
		}
	}

	return nil
}

func (a *MetricAction) AddCustomTemplate(customTemplate *exporterTemplate) error {

	if err := AddCustomTemplate(a, customTemplate); err != nil {
		return err
	}

	return nil
}
