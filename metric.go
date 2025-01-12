package main

import (
	"fmt"
	"log/slog"
	"reflect"
	"sort"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"google.golang.org/protobuf/proto"
)

// MetricDesc is a descriptor for a family of metrics, sharing the same name, help, labes, type.
type MetricDesc interface {
	Name() string
	Help() string
	ValueType() prometheus.ValueType
	ConstLabels() []*dto.LabelPair
	// Labels() []string
	LogContext() []interface{}
}

//
// MetricFamily
//

// MetricFamily implements MetricDesc for SQL metrics, with logic for populating its labels and values from sql.Rows.
type MetricFamily struct {
	config       *MetricConfig
	name         string
	help         string
	constLabels  []*dto.LabelPair
	labels       []*Label // raw string or template
	valueslabels []*Label // raw string or template for key and value
	logContext   []interface{}
}

// NewMetricFamily creates a new MetricFamily with the given metric config and const labels (e.g. job and instance).
func NewMetricFamily(
	logContext []interface{},
	mc *MetricConfig,
	constLabels []*dto.LabelPair,
	customTemplate *exporterTemplate) (*MetricFamily, error) {

	var (
		err    error
		labels []*Label
	)

	logContext = append(logContext, "metric", mc.Name)

	if len(mc.Values) == 0 {
		logContext = append(logContext, "errmsg", "NewMetricFamily(): multiple values but no value label")
		return nil, fmt.Errorf("%s", logContext...)
	}
	if len(mc.Values) > 1 && len(mc.ValueLabel) == 0 {
		logContext = append(logContext, "errmsg", "NewMetricFamily(): multiple values but no value label")
	}

	// all labels are stored in variable 'labels': size of slice if size of KeyLabels + 1 if size of Values is greater than 1
	if len(mc.key_labels_map) > 0 {
		label_len := len(mc.key_labels_map)
		if len(mc.Values) > 1 {
			label_len++
		}
		labels = make([]*Label, label_len)

		i := 0
		for key, val := range mc.key_labels_map {
			labels[i], err = NewLabel(key, val, mc.Name, "key_label", customTemplate)
			if err != nil {
				return nil, err
			}
			i++
		}
		// add an element in labels for value_label (the name of the label); value will be set later
		if len(mc.Values) > 1 {
			labels[i], err = NewLabel(mc.ValueLabel, "", mc.Name, "value_label", customTemplate)
			if err != nil {
				return nil, err
			}
		}
	}
	// values are stored in variable 'valueslabels': it is meanfull only when values are greater than 1.
	// original config struct is a map of [value label] : [value template];
	valueslabels := make([]*Label, len(mc.Values))

	i := 0
	for key, val := range mc.Values {
		valueslabels[i], err = NewLabel(key, val, mc.Name, "'values'", customTemplate)
		if err != nil {
			return nil, err
		}
		i++
	}

	// Create a copy of original slice to avoid modifying constLabels
	sortedLabels := append(constLabels[:0:0], constLabels...)

	for k, v := range mc.StaticLabels {
		sortedLabels = append(sortedLabels, &dto.LabelPair{
			Name:  proto.String(k),
			Value: proto.String(v),
		})
	}
	sort.Sort(labelPairSorter(sortedLabels))

	return &MetricFamily{
		config:       mc,
		constLabels:  sortedLabels,
		labels:       labels,
		valueslabels: valueslabels,
		logContext:   logContext,
	}, nil
}

// Collect is the equivalent of prometheus.Collector.Collect() but takes a Query output map to populate values from.
func (mf MetricFamily) Collect(rawdatas any, logger *slog.Logger, ch chan<- Metric) {
	var set_root bool = false
	// reset logcontxt for MetricFamily: remove previous errors if any
	mf.logContext = make([]any, 2)
	mf.logContext[0] = "metric"
	mf.logContext[1] = mf.config.Name

	item, ok := rawdatas.(map[string]any)
	if !ok {
		ch <- NewInvalidMetric(mf.logContext, fmt.Errorf("metric %s symbols table", mf.config.Name))
		return
	}
	if mf.name == "" {
		name, err := mf.config.name.GetValueString(item, nil, false)
		if err != nil {
			ch <- NewInvalidMetric(mf.logContext, fmt.Errorf("metric %s can't get metric name", mf.config.Name))
			return
		} else {
			mf.name = name
		}
	}
	if mf.help == "" {
		help, err := mf.config.help.GetValueString(item, nil, false)
		if err != nil {
			ch <- NewInvalidMetric(mf.logContext, fmt.Errorf("metric %s can't get metric help", mf.name))
			return
		} else {
			mf.help = help
		}
	}
	mf.logContext[1] = mf.name

	// set default scope to "loop_var" entry: item[item["loop_var"]]
	if mf.config.Scope == "" {
		if loop_var, ok := item["loop_var"].(string); ok {
			if loop_var == "empty_item" {
				loop_var = "item"
			}
			if datas, ok := item[loop_var].(map[string]any); ok {
				item = datas
				item["root"] = rawdatas
				set_root = true
			}
		}
	} else if mf.config.Scope != "none" {
		var err error
		item, err = SetScope(mf.config.Scope, item)
		if err != nil {
			ch <- NewInvalidMetric(mf.logContext, err)
			return
		}
		item["root"] = rawdatas
		set_root = true
	}

	// build the labels family with the content of the var(*Field)
	if len(mf.labels) == 0 && mf.config.key_labels != nil {
		if labelsmap_raw, err := ValorizeValue(item, mf.config.key_labels, logger, mf.name, true); err == nil {
			if key_labels_map, ok := labelsmap_raw.(map[string]any); ok {
				label_len := len(key_labels_map)
				if len(mf.config.Values) > 1 {
					label_len++
				}
				mf.labels = make([]*Label, label_len)

				i := 0
				for key, val_raw := range key_labels_map {
					if val, ok := val_raw.(string); ok {
						mf.labels[i], err = NewLabel(key, val, mf.name, "key_label", nil)
						if err != nil {
							logger.Warn(fmt.Sprintf("invalid template for key_values for metric %s: %s (maybe use |toRawJson.)", mf.name, err))
							continue
						}
					}
					i++
				}
				// add an element in labels for value_label (the name of the label); value will be set later
				if len(mf.config.Values) > 1 {
					mf.labels[i], _ = NewLabel(mf.config.ValueLabel, "", mf.name, "value_label", nil)
					if err != nil {
						logger.Warn(fmt.Sprintf("invalid templatefor value_label for metric %s: %s (maybe use |toRawJson.)", mf.name, err))
						// 	return nil, err
					}
				}
			} else {
				logger.Warn("invalid type need map[string][string]", "type_error", reflect.TypeOf(labelsmap_raw))
			}
		} else {
			logger.Warn(fmt.Sprintf("invalid template for key_values for metric %s: %s (maybe use |toRawJson.)", mf.name, err))
		}
	}

	labelNames := make([]string, len(mf.labels))
	labelValues := make([]string, len(mf.labels))

	for i, label := range mf.labels {
		var err error
		// last label may be null if metric has no value label, because it has only one value
		if label == nil {
			continue
		}
		if item != nil {
			if label.Key != nil {
				labelNames[i], err = label.Key.GetValueString(item, nil, false)
				if err != nil {
					ch <- NewInvalidMetric(mf.logContext, fmt.Errorf("invalid template label name metric{ name: %s, key: %s} : %s", mf.name, label.Key.String(), err))
					continue
				}
			}

			if label.Value != nil {
				sub := make(map[string]string)
				sub["_"] = labelNames[i]
				labelValues[i], err = label.Value.GetValueString(item, sub, true)
				if err != nil {
					ch <- NewInvalidMetric(mf.logContext, fmt.Errorf("invalid template label value metric{ name: %s, value: %s} : %s", mf.name, label.Value.String(), err))
					continue
				}
			}
		} else {
			labelNames[i] = fmt.Sprintf("undef_%d", i)
			labelValues[i] = ""
		}
		i++
	}

	for _, label := range mf.valueslabels {
		var f_value float64
		var err error

		// fill the label value for value_label with the current name of the value if there is a label !
		if len(mf.config.Values) > 1 {
			labelValues[len(labelValues)-1], err = label.Key.GetValueString(item, nil, false)
			if err != nil {
				ch <- NewInvalidMetric(mf.logContext, fmt.Errorf("invalid template label name metric{ name: %s, key: %s} : %s", mf.name, label.Key.String(), err))
				continue
			}
		}
		f_value, err = label.Value.GetValueFloat(item)
		if err != nil {
			ch <- NewInvalidMetric(mf.logContext, fmt.Errorf("invalid template label value metric{ name: %s, value: %s} : %s", mf.name, label.Value.String(), err))
			continue
		}

		ch <- NewMetric(&mf, f_value, labelNames, labelValues)
	}
	if set_root {
		delete(item, "root")
	}
}

// Name implements MetricDesc.
func (mf MetricFamily) Name() string {
	name := mf.name
	if name == "" {
		name = mf.config.Name
	}
	return name
}

// Help implements MetricDesc.
func (mf MetricFamily) Help() string {
	help := mf.help
	if help == "" {
		help = mf.config.Help
	}
	return help
}

// ValueType implements MetricDesc.
func (mf MetricFamily) ValueType() prometheus.ValueType {
	return mf.config.ValueType()
}

// ConstLabels implements MetricDesc.
func (mf MetricFamily) ConstLabels() []*dto.LabelPair {
	return mf.constLabels
}

// LogContext implements MetricDesc.
func (mf MetricFamily) LogContext() []interface{} {
	return mf.logContext
}

//
// automaticMetricDesc
//

// automaticMetric is a MetricDesc for automatically generated metrics (e.g. `up` and `scrape_duration`).
type automaticMetricDesc struct {
	name        string
	help        string
	valueType   prometheus.ValueType
	labels      []string
	constLabels []*dto.LabelPair
	logContext  []interface{}
}

// NewAutomaticMetricDesc creates a MetricDesc for automatically generated metrics.
func NewAutomaticMetricDesc(
	logContext []interface{}, name, help string, valueType prometheus.ValueType, constLabels []*dto.LabelPair, labels ...string) MetricDesc {
	return &automaticMetricDesc{
		name:        name,
		help:        help,
		valueType:   valueType,
		constLabels: constLabels,
		labels:      labels,
		logContext:  logContext,
	}
}

// Name implements MetricDesc.
func (a automaticMetricDesc) Name() string {
	return a.name
}

// Help implements MetricDesc.
func (a automaticMetricDesc) Help() string {
	return a.help
}

// ValueType implements MetricDesc.
func (a automaticMetricDesc) ValueType() prometheus.ValueType {
	return a.valueType
}

// ConstLabels implements MetricDesc.
func (a automaticMetricDesc) ConstLabels() []*dto.LabelPair {
	return a.constLabels
}

// Labels implements MetricDesc.
func (a automaticMetricDesc) Labels() []string {
	return a.labels
}

// LogContext implements MetricDesc.
func (a automaticMetricDesc) LogContext() []interface{} {
	return a.logContext
}

//
// Metric
//

// A Metric models a single sample value with its meta data being exported to Prometheus.
type Metric interface {
	Desc() MetricDesc
	Write(out *dto.Metric) error
}

// NewMetric returns a metric with one fixed value that cannot be changed.
//
// NewMetric panics if the length of labelValues is not consistent with desc.labels().
func NewMetric(desc MetricDesc, value float64, labelNames []string, labelValues []string) Metric {
	if len(labelNames) != len(labelValues) {
		panic(fmt.Sprintf("[%s] expected %d labels, got %d", desc.LogContext(), len(labelNames), len(labelValues)))
	}
	return &constMetric{
		desc:       desc,
		val:        value,
		labelPairs: makeLabelPairs(desc, labelNames, labelValues),
	}
}

// constMetric is a metric with one fixed value that cannot be changed.
type constMetric struct {
	desc       MetricDesc
	val        float64
	labelPairs []*dto.LabelPair
}

// Desc implements Metric.
func (m *constMetric) Desc() MetricDesc {
	return m.desc
}

// Write implements Metric.
func (m *constMetric) Write(out *dto.Metric) error {
	out.Label = m.labelPairs
	switch t := m.desc.ValueType(); t {
	case prometheus.CounterValue:
		out.Counter = &dto.Counter{Value: proto.Float64(m.val)}
	case prometheus.GaugeValue:
		out.Gauge = &dto.Gauge{Value: proto.Float64(m.val)}
	default:
		var logContext []interface{}
		logContext = append(logContext, m.desc.LogContext()...)
		logContext = append(logContext, "errmsg", fmt.Sprintf("encountered unknown type %v", t))
		return fmt.Errorf("%s", logContext...)
	}
	return nil
}

func makeLabelPairs(desc MetricDesc, labelNames []string, labelValues []string) []*dto.LabelPair {
	labels := labelNames
	constLabels := desc.ConstLabels()

	totalLen := len(labels) + len(constLabels)
	if totalLen == 0 {
		// Super fast path.
		return nil
	}
	if len(labels) == 0 {
		// Moderately fast path.
		return constLabels
	}
	labelPairs := make([]*dto.LabelPair, 0, totalLen)
	for i, label := range labels {
		labelPairs = append(labelPairs, &dto.LabelPair{
			Name:  proto.String(label),
			Value: proto.String(labelValues[i]),
		})
	}
	labelPairs = append(labelPairs, constLabels...)
	sort.Sort(labelPairSorter(labelPairs))
	return labelPairs
}

// labelPairSorter implements sort.Interface.
// It provides a sortable version of a slice of dto.LabelPair pointers.

type labelPairSorter []*dto.LabelPair

func (s labelPairSorter) Len() int {
	return len(s)
}

func (s labelPairSorter) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s labelPairSorter) Less(i, j int) bool {
	return s[i].GetName() < s[j].GetName()
}

type invalidMetric struct {
	logContext []interface{}
	err        error
}

// NewInvalidMetric returns a metric whose Write method always returns the provided error.
func NewInvalidMetric(logContext []interface{}, err error) Metric {
	return invalidMetric{
		logContext: logContext,
		err:        err,
	}
}

func (m invalidMetric) Desc() MetricDesc { return nil }

func (m invalidMetric) Write(*dto.Metric) error {
	return m.err
}

type Label struct {
	Key   *Field
	Value *Field
}

func NewLabel(key string, value string, mName string, errStr string, customTemplate *exporterTemplate) (*Label, error) {
	var (
		keyField, valueField *Field
		err                  error
	)

	keyField, err = NewField(key, customTemplate)
	if err != nil {
		return nil, fmt.Errorf("NewMetricFamily(): name of %s for metric %q: %s", errStr, mName, err)
	}
	if value != "" {
		valueField, err = NewField(value, customTemplate)
		if err != nil {
			return nil, fmt.Errorf("NewMetricFamily(): value of %s for metric %q: %s", errStr, mName, err)
		}
	}

	return &Label{
		Key:   keyField,
		Value: valueField,
	}, nil
}
