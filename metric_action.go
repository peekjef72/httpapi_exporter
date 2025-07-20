// cSpell:ignore histo

package main

import (
	//"bytes"
	"fmt"
	"log/slog"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

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

func (a *MetricAction) GetWidth() []any {
	return a.With
}
func (a *MetricAction) SetWidth(with []any) {
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
func (a *MetricAction) SetLoopVar(loopvar string) {
	a.LoopVar = loopvar
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
	nameField *Field,
	vars [][]any,
	with []any,
	loopVar string,
	when []*Field,
	until []*Field) error {
	return setBasicElement(a, nameField, vars, with, loopVar, when, until)
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

	if a.metricFamily.ValueType() == dto.MetricType_HISTOGRAM {
		var histo *prometheus.HistogramVec
		if len(a.metricFamily.config.histogram.Histogram) > loop_var_idx {
			histo = a.metricFamily.config.histogram.Histogram[loop_var_idx]
		}
		symtab["__histogram"] = histo
	}
	a.metricFamily.Collect(symtab, logger, metric_channel)
	if a.metricFamily.ValueType() == dto.MetricType_HISTOGRAM {
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
