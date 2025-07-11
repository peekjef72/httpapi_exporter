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
			err   error
			scope string
		)
		if mf.config.Scope == "" {
			if loop_var, ok := symtab["loop_var"].(string); ok {
				if loop_var == "empty_item" {
					loop_var = "item"
				}
				scope = loop_var
			}
		} else {
			scope = mf.config.Scope
		}
		if scope != "" {
			logger.Debug(fmt.Sprintf("metric.Collect() scope set to '%s'", scope))
			symtab, err = SetScope(scope, symtab)
			if err != nil {
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
			ch <- NewInvalidMetric(mf.logContext, err)
			return
		} else {
			mf.help = help
		}
	}
	mf.logContext[1] = mf.name

	// build the labels family with the content of the var(*Field)
	if len(mf.labels) == 0 && mf.config.key_labels != nil {
		if labelsmap_raw, err := ValorizeValue(symtab, mf.config.key_labels, logger, mf.name, true); err == nil {
			if key_labels_map, ok := labelsmap_raw.(map[string]any); ok {
				label_len := len(key_labels_map)
				if len(mf.config.Values) > 1 {
					label_len++
				}
				mf.labels = make([]*Label, label_len)

				i := 0
				for key, val_raw := range key_labels_map {
					mf.labels[i], err = NewLabel(key, RawGetValueString(val_raw), mf.name, "key_label", nil)
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
