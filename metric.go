package main

import (
	"fmt"
	"log/slog"
	"reflect"
	"sort"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/spf13/cast"
	"google.golang.org/protobuf/proto"
)

// MetricDesc is a descriptor for a family of metrics, sharing the same name, help, labes, type.
type MetricDesc interface {
	Name() string
	Help() string
	ValueType() dto.MetricType
	ConstLabels() []*dto.LabelPair
	// Labels() []string
	LogContext() []interface{}
	Config() *MetricConfig
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

	if mc.valueType != dto.MetricType_HISTOGRAM && len(mc.Values) == 0 {
		logContext = append(logContext, "errmsg", "NewMetricFamily(): no value defined")
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
	var (
		set_root bool = false
	)
	// reset logcontxt for MetricFamily: remove previous errors if any
	mf.logContext = make([]any, 2)
	mf.logContext[0] = "metric"
	mf.logContext[1] = mf.config.Name

	symtab, ok := rawdatas.(map[string]any)
	if !ok {
		err := fmt.Errorf("metric %s symbols table undefined", mf.config.Name)
		logger.Warn(err.Error())
		ch <- NewInvalidMetric(mf.logContext, err)
		return
	}
	//	symtab := item
	logger.Debug("metric.Collect() check scope",
		"coll", CollectorId(symtab, logger),
		"script", ScriptName(symtab, logger),
	)
	root_symtab := symtab
	// set default scope to "loop_var" entry: item[item["loop_var"]]
	if mf.config.Scope == "" || mf.config.Scope != "none" {
		var (
			err            error
			scope          string
			implicit_scope bool = false
		)
		if mf.config.Scope == "" {
			if loop_var, ok := symtab["loop_var"].(string); ok {
				if loop_var == "empty_item" {
					loop_var = "item"
				}
				scope = loop_var
				implicit_scope = true
			}
		} else {
			scope = mf.config.Scope
		}
		if scope != "" {
			logger.Debug(fmt.Sprintf("metric.Collect() scope set to '%s'", scope))
			symtab, err = SetScope(scope, symtab)
			if err != nil && !implicit_scope {
				err = fmt.Errorf("metric %s: %s", mf.name, err.Error())
				logger.Warn(err.Error(),
					"coll", CollectorId(root_symtab, logger),
					"script", ScriptName(root_symtab, logger),
				)
				// only stop if scope is not found!
				if sc_err, ok := err.(ScopeError); ok {
					if sc_err.Code() == error_scope_not_found {
						ch <- NewInvalidMetric(mf.logContext, err)
						return
					}
				}
				// else try to continue... recover error.
			}
			// set root entry in symtab if not already exists.
			// don't allow several levels of root
			if _, found := symtab["root"]; !found {
				symtab["root"] = rawdatas
				set_root = true
			}
		}
	} else {
		logger.Debug("metric.Collect() scope set to 'none'",
			"coll", CollectorId(root_symtab, logger),
			"script", ScriptName(root_symtab, logger),
		)
	}

	if mf.name == "" {
		name, err := mf.config.name.GetValueString(symtab, logger)
		if err != nil {
			err := fmt.Errorf("metric %s can't get metric name", mf.config.Name)
			logger.Warn(err.Error(),
				"coll", CollectorId(root_symtab, logger),
				"script", ScriptName(root_symtab, logger),
			)
			ch <- NewInvalidMetric(mf.logContext, err)
			return
		} else {
			mf.name = name
		}
	}
	if mf.help == "" {
		help, err := mf.config.help.GetValueString(symtab, logger)
		if err != nil {
			err := fmt.Errorf("metric %s can't get metric help", mf.name)
			logger.Warn(err.Error(),
				"coll", CollectorId(root_symtab, logger),
				"script", ScriptName(root_symtab, logger),
			)
			// don't remove metric it can't obtain help !
			// ch <- NewInvalidMetric(mf.logContext, err)
			// return
		} else {
			mf.help = help
		}
	}
	mf.logContext[1] = mf.name

	// build the labels family with the content of the var(*Field)
	if len(mf.labels) == 0 && mf.config.key_labels != nil {
		if labelsmap_raw, err := ValorizeValue(symtab, mf.config.key_labels, logger, mf.name, true); err == nil {
			t_labels := reflect.ValueOf(labelsmap_raw)
			if t_labels.Kind() == reflect.Map {
				label_len := t_labels.Len()
				// }
				// if key_labels_map, ok := labelsmap_raw.(map[string]any); ok {
				// 	label_len := len(key_labels_map)
				if len(mf.config.Values) > 1 {
					label_len++
				}
				mf.labels = make([]*Label, label_len)

				i := 0
				iter := t_labels.MapRange()
				for iter.Next() {
					raw_key := iter.Key().Interface()
					raw_value := iter.Value()
					// for key, val_raw := range key_labels_map {
					// mf.labels[i], err = NewLabel(key, RawGetValueString(val_raw), mf.name, "key_label", nil)
					mf.labels[i], err = NewLabel(
						RawGetValueString(raw_key),
						RawGetValueString(raw_value),
						mf.name, "key_label", nil)
					if err != nil {
						logger.Warn(fmt.Sprintf("invalid template for key_values for metric %s: %s (maybe use |toRawJson.)", mf.name, err),
							"coll", CollectorId(root_symtab, logger),
							"script", ScriptName(root_symtab, logger),
						)
						continue
					}
					i++
				}
				// add an element in labels for value_label (the name of the label); value will be set later
				if len(mf.config.Values) > 1 {
					mf.labels[i], err = NewLabel(mf.config.ValueLabel, "", mf.name, "value_label", nil)
					if err != nil {
						logger.Warn(fmt.Sprintf("invalid templatefor value_label for metric %s: %s (maybe use |toRawJson.)", mf.name, err),
							"coll", CollectorId(root_symtab, logger),
							"script", ScriptName(root_symtab, logger),
						)
						// 	return nil, err
					}
				}
			} else {
				logger.Warn("invalid type need map[string][string]", "type_error", reflect.TypeOf(labelsmap_raw),
					"coll", CollectorId(root_symtab, logger),
					"script", ScriptName(root_symtab, logger),
				)
			}
		} else {
			logger.Warn(
				fmt.Sprintf("invalid template for key_values for metric %s: %s (maybe use |toRawJson.)", mf.name, err),
				"coll", CollectorId(root_symtab, logger),
				"script", ScriptName(root_symtab, logger),
			)
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
		if symtab != nil {
			if label.Key != nil {
				labelNames[i], err = label.Key.GetValueString(symtab, logger)
				if err != nil {
					err = fmt.Errorf("invalid template label name metric{ name: %s, key: %s} : %s", mf.name, label.Key.String(), err)
					logger.Warn(err.Error())
					ch <- NewInvalidMetric(mf.logContext, err)
					return
				}
			}

			if label.Value != nil {
				labelValues[i], err = label.Value.GetValueString(symtab, logger)
				if err != nil {
					err = fmt.Errorf("invalid template label value metric{ name: %s, value: %s} : %s", mf.name, label.Value.String(), err)
					logger.Warn(err.Error(),
						"coll", CollectorId(root_symtab, logger),
						"script", ScriptName(root_symtab, logger),
					)
					ch <- NewInvalidMetric(mf.logContext, err)
					return
				}
			}
		} else {
			labelNames[i] = fmt.Sprintf("undef_%d", i)
			labelValues[i] = ""
		}
		i++
	}

	if mf.config.valueType == dto.MetricType_HISTOGRAM {
		switch mf.config.histogram.Type {
		case HistogramTypeExternal:
			met, _ := NewHistogramMetric(&mf, labelNames, labelValues, nil)
			h_var, err := ValorizeValue(symtab, mf.config.histogram.Histogram_var, logger, mf.name, false)
			if err != nil {
				ch <- NewInvalidMetric(mf.logContext, err)
			} else {

				if err := met.SetValue(h_var); err != nil {
					ch <- NewInvalidMetric(mf.logContext, err)
				}
				ch <- met
			}
		case HistogramTypeStatic:
			// if mf.config.histogram.Histogram == nil {
			// 	mf.config.histogram.Histogram = prometheus.NewHistogramVec(
			// 		prometheus.HistogramOpts{
			// 			Name:    "set_later",
			// 			Help:    "set_later",
			// 			Buckets: *mf.config.histogram.Buckets,
			// 		}, []string{})
			// }
			// try to find a previously existing HistogramVec
			var histogram *prometheus.HistogramVec
			if r_val, ok := root_symtab["__histogram"]; ok {
				if histo, ok := r_val.(*prometheus.HistogramVec); ok {
					histogram = histo
				}
			}
			met, histo := NewHistogramMetric(&mf, labelNames, labelValues, histogram)
			if histo != histogram {
				root_symtab["__histogram"] = histo
			}
			f_value, err := mf.config.histogram.Histogram_value.GetValueFloat(symtab, logger)
			if err != nil {
				ch <- NewInvalidMetric(mf.logContext, err)
			} else {
				if err := met.SetValue(f_value); err != nil {
					ch <- NewInvalidMetric(mf.logContext, err)
				}
				ch <- met
			}
		}
	} else {
		for _, label := range mf.valueslabels {
			var f_value float64
			var err error

			// fill the label value for value_label with the current name of the value if there is a label !
			if len(mf.config.Values) > 1 {
				labelValues[len(labelValues)-1], err = label.Key.GetValueString(symtab, logger)
				if err != nil {
					err = fmt.Errorf("invalid template label name metric{ name: %s, key: %s} : %s", mf.name, label.Key.String(), err)
					logger.Warn(err.Error(),
						"coll", CollectorId(root_symtab, logger),
						"script", ScriptName(root_symtab, logger),
					)
					ch <- NewInvalidMetric(mf.logContext, err)
					return
				}
			}
			f_value, err = label.Value.GetValueFloat(symtab, logger)
			if err != nil {
				warn := true
				if var_err, ok := err.(VarError); ok {
					if var_err.Code() == error_var_not_found {
						warn = false
						logger.Debug(err.Error(),
							"coll", CollectorId(root_symtab, logger),
							"script", ScriptName(root_symtab, logger),
						)
					}
				}
				// do not complain in logs if metric was not found.
				if warn {
					err = fmt.Errorf("invalid template label value metric{ name: %s, value: %s} : %s", mf.name, label.Value.String(), err)
					logger.Warn(err.Error(),
						"coll", CollectorId(root_symtab, logger),
						"script", ScriptName(root_symtab, logger),
					)
					ch <- NewInvalidMetric(mf.logContext, err)
				}
				return
			}
			logger.Debug(
				fmt.Sprintf("metric.Collect() send metric to channel (len labelNames: %d lenlabelValue: %d)", len(labelNames), len(labelValues)),
				"coll", CollectorId(root_symtab, logger),
				"script", ScriptName(root_symtab, logger),
			)
			ch <- NewMetric(&mf, f_value, labelNames, labelValues)
		}
	}
	if set_root {
		delete(symtab, "root")
		// logger.Debug(
		// 	"[metric.Collect()] remove root from symtab",
		// 	"coll", CollectorId(symtab, logger),
		// 	"script", ScriptName(symtab, logger),
		// )
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
func (mf MetricFamily) ValueType() dto.MetricType {
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

// Config implements MetricDesc.
func (mf MetricFamily) Config() *MetricConfig {
	return mf.config
}

//
// automaticMetricDesc
//

// automaticMetric is a MetricDesc for automatically generated metrics (e.g. `up` and `scrape_duration`).
type automaticMetricDesc struct {
	name        string
	help        string
	valueType   dto.MetricType
	labels      []string
	constLabels []*dto.LabelPair
	logContext  []interface{}
}

// NewAutomaticMetricDesc creates a MetricDesc for automatically generated metrics.
func NewAutomaticMetricDesc(
	logContext []interface{}, name, help string, valueType dto.MetricType, constLabels []*dto.LabelPair, labels ...string) MetricDesc {
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
func (a automaticMetricDesc) ValueType() dto.MetricType {
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

// Config implements MetricDesc.
func (a automaticMetricDesc) Config() *MetricConfig { return nil }

//
// Metric
//

// A Metric models a single sample value with its meta data being exported to Prometheus.
type Metric interface {
	Desc() MetricDesc
	Write(out *dto.Metric) error
	SetValue(any) error
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
	case dto.MetricType_COUNTER:
		out.Counter = &dto.Counter{Value: proto.Float64(m.val)}
	case dto.MetricType_GAUGE:
		out.Gauge = &dto.Gauge{Value: proto.Float64(m.val)}
	default:
		var logContext []interface{}
		logContext = append(logContext, m.desc.LogContext()...)
		logContext = append(logContext, "errmsg", fmt.Sprintf("encountered unknown type %v", t))
		return fmt.Errorf("%s", logContext...)
	}
	return nil
}

// SetValue implements Metric.
func (m *constMetric) SetValue(val any) error { return nil }

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

// SetValue implements Metric.
func (m invalidMetric) SetValue(val any) error { return nil }

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

type histMetric struct {
	desc      MetricDesc
	metric    dto.Metric
	histogram *prometheus.HistogramVec
}

// Desc implements Metric.
func (m *histMetric) Desc() MetricDesc {
	return m.desc
}

// Write implements Metric.
func (m *histMetric) Write(out *dto.Metric) error {
	out.Label = m.metric.GetLabel()
	out.Histogram = m.metric.GetHistogram()
	return nil
}

func NewHistogramMetric(
	desc MetricDesc,
	labelNames []string,
	labelValues []string,
	histogram *prometheus.HistogramVec) (Metric, *prometheus.HistogramVec) {

	if len(labelNames) != len(labelValues) {
		panic(fmt.Sprintf("[%s] expected %d labels, got %d", desc.LogContext(), len(labelNames), len(labelValues)))
	}

	config := desc.Config()
	if config.histogram.Type == HistogramTypeStatic && histogram == nil {
		// check if the metric config has already the histogram definition
		histogram = prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "set_later",
				Help:    "set_later",
				Buckets: *config.histogram.Buckets,
			}, labelNames)
	}
	return &histMetric{
		desc: desc,
		metric: dto.Metric{
			Label: makeLabelPairs(desc, labelNames, labelValues),
		},
		histogram: histogram,
	}, histogram
}

// SetValue implements Metric.
func (m *histMetric) SetValue(hist_var_raw any) error {

	config := m.desc.Config()
	switch config.histogram.Type {
	case HistogramTypeExternal:
		h_var, ok := hist_var_raw.(map[string]any)
		if !ok {
			return fmt.Errorf("invalid value transmit: must be map[string]any")
		}
		m.metric.Histogram = &dto.Histogram{}
		hist := m.metric.GetHistogram()
		if raw_count, ok := h_var["sample_count"]; ok {
			count := cast.ToUint64(raw_count)
			hist.SampleCount = &count
		}
		if raw_sum, ok := h_var["sample_sum"]; ok {
			sum := cast.ToFloat64(raw_sum)
			hist.SampleSum = &sum
		}
		if raw_buckets, ok := h_var["buckets"]; ok {
			t_buckets := reflect.ValueOf(raw_buckets)
			if t_buckets.Kind() == reflect.Slice {
				for ind := range t_buckets.Len() {
					raw_bucket := t_buckets.Index(ind).Interface()
					t_raw_bucket := reflect.ValueOf(raw_bucket)
					if t_raw_bucket.Kind() == reflect.Map {
						iter := t_raw_bucket.MapRange()
						var (
							count uint64
							bound float64
						)
						for iter.Next() {
							raw_key := iter.Key()
							if raw_key.Kind() == reflect.String {
								key := raw_key.String()
								switch key {
								case "le":
									bound = cast.ToFloat64(iter.Value().Interface())
								case "value":
									count = cast.ToUint64(iter.Value().Interface())
								}
							}
						}
						new_bucket := &dto.Bucket{
							CumulativeCount: &count,
							UpperBound:      &bound,
						}
						hist.Bucket = append(hist.Bucket, new_bucket)
					}
				}
			}
		}
	case HistogramTypeStatic:
		if f_value, ok := hist_var_raw.(float64); ok {
			labels := m.metric.GetLabel()
			var values []string
			for _, value := range labels {
				values = append(values, value.GetValue())
			}
			obs, err := m.histogram.GetMetricWithLabelValues(values...)
			if err != nil {
				return err
			}
			obs.Observe(f_value)
			loc_ch := make(chan prometheus.Metric, 10)
			m.histogram.Collect(loc_ch)
			loc_met := <-loc_ch
			loc_met.Write(&m.metric)
			m.metric.Label = labels
		}
	}
	return nil
}

// func BuildHistogram(h_obj map[string]any) *dto.MetricFamily {
// 	mf := &dto.MetricFamily{}

// 	name := GetMapValueString(h_obj,"name")
// 	mf.Name = &name
// 	help := GetMapValueString(h_obj,"help")
// 	mf.Help = &help
// 	typeH := dto.MetricType_HISTOGRAM
// 	mf.Type = &typeH
// 	for _, metric_obj_raw := range GetMapValueSlice(h_obj, "metrics") {
// 		if metric_obj, ok := metric_obj_raw.(map[string]any); ok {
// 			labels := GetMapValueMap(metric_obj, "labels")
// 			for key, val := range labels {
// 				metric.Label =
// 			}
// 		}
// 		a inclure dans Collect
// 	}
// 	return mf
// }
