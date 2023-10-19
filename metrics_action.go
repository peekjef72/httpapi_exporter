package main

import (
	//"bytes"
	"fmt"
	"strings"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
)

// ***************************************************************************************
// ***************************************************************************************
// metrics action (block / loop )
// ***************************************************************************************
// ***************************************************************************************

type MetricsAction struct {
	// BaseAction
	Name    *Field              `yaml:"name,omitempty"`
	With    []any               `yaml:"with,omitempty"`
	When    []*exporterTemplate `yaml:"when,omitempty"`
	LoopVar string              `yaml:"loop_var,omitempty"`
	Vars    map[string]any      `yaml:"vars,omitempty"`
	Until   []*exporterTemplate `yaml:"until,omitempty"`

	Metrics      []*MetricConfig `yaml:"metrics"`                 // metrics defined by this collector
	Scope        string          `yaml:"scope,omitempty"`         // var path where to collect data: shortcut for {{ .scope.path.var }}
	MetricPrefix string          `yaml:"metric_prefix,omitempty"` // var to alert metric name

	// Catches all undefined fields and must be empty after parsing.
	XXX     map[string]interface{} `yaml:",inline" json:"-"`
	Actions []Action               `yaml:"-"`
}

func (a *MetricsAction) Type() int {
	return metrics_action
}

func (a *MetricsAction) GetName(symtab map[string]any, logger log.Logger) string {
	str, err := a.Name.GetValueString(symtab, nil, false)
	if err != nil {
		level.Warn(logger).Log(
			"collid", CollectorId(symtab, logger),
			"script", ScriptName(symtab, logger),
			"msg", fmt.Sprintf("invalid action name: %v", err))
		return ""
	}
	return str
}

func (a *MetricsAction) GetNameField() *Field {
	return a.Name
}
func (a *MetricsAction) SetNameField(name *Field) {
	a.Name = name
}

func (a *MetricsAction) GetWidh() []any {
	return a.With
}
func (a *MetricsAction) SetWidth(with []any) {
	a.With = with
}

func (a *MetricsAction) GetWhen() []*exporterTemplate {
	return a.When

}
func (a *MetricsAction) SetWhen(when []*exporterTemplate) {
	a.When = when
}

func (a *MetricsAction) GetLoopVar() string {
	return a.LoopVar
}
func (a *MetricsAction) SetLoopVar(loopvar string) {
	a.LoopVar = loopvar
}

func (a *MetricsAction) GetVars() map[string]any {
	return a.Vars
}
func (a *MetricsAction) SetVars(vars map[string]any) {
	a.Vars = vars
}

func (a *MetricsAction) GetUntil() []*exporterTemplate {
	return a.Until
}
func (a *MetricsAction) SetUntil(until []*exporterTemplate) {
	a.Until = until
}

func (a *MetricsAction) setBasicElement(
	nameField *Field,
	vars map[string]any,
	with []any,
	loopVar string,
	when []*exporterTemplate,
	until []*exporterTemplate) error {
	return setBasicElement(a, nameField, vars, with, loopVar, when, until)
}

func (a *MetricsAction) PlayAction(script *YAMLScript, symtab map[string]any, logger log.Logger) error {
	return PlayBaseAction(script, symtab, logger, a, a.CustomAction)
}

// only for MetricsAction
func (a *MetricsAction) GetMetrics() []*GetMetricsRes {
	res := make([]*GetMetricsRes, 1)
	res[0] = &GetMetricsRes{
		mc:       a.Metrics,
		maprefix: a.MetricPrefix,
	}
	return res
}

// only for MetricAction
func (a *MetricsAction) GetMetric() *MetricConfig {
	return nil
}
func (a *MetricsAction) SetMetricFamily(*MetricFamily) {
}

// only for PlayAction
func (a *MetricsAction) SetPlayAction(scripts map[string]*YAMLScript) error {
	return nil
}

// ***************************************************************************************
// specific behavior for the MetricsAction
func SetScope(scope string, symtab map[string]any) (map[string]any, error) {
	var err error

	tmp_symtab := symtab
	// split the scope string into parts: attr1.attr[0].attr
	if scope[0] == '.' {
		scope = scope[1:]
	}
	vars := strings.Split(scope, ".")
	// last_elmt := len(vars) -1
	for _, var_name := range vars {
		if raw_value, ok := tmp_symtab[var_name]; ok {
			switch cur_value := raw_value.(type) {
			case map[string]any:
				tmp_symtab = cur_value
			default:
				err = fmt.Errorf("can't set scope: '%s' has invalid type", var_name)
			}
			// }
		} else {
			err = fmt.Errorf("can't set scope: '%s' not found", var_name)
		}
	}
	return tmp_symtab, err
}

// ***************************************************************************************
func (a *MetricsAction) CustomAction(script *YAMLScript, symtab map[string]any, logger log.Logger) error {

	var (
		metric_channel chan<- Metric
		// mfs            []*MetricFamily
	)
	// var logContext []any

	level.Debug(logger).Log(
		"collid", CollectorId(symtab, logger),
		"script", ScriptName(symtab, logger),
		"msg", fmt.Sprintf("[Type: MetricsAction] Name: %s - %d metrics_name to set", a.GetName(symtab, logger), len(a.Metrics)))

	query_status, ok := GetMapValueBool(symtab, "query_status")
	if !ok || (ok && !query_status) {
		level.Debug(logger).Log(
			"collid", CollectorId(symtab, logger),
			"script", ScriptName(symtab, logger),
			"msg", fmt.Sprintf("[Type: MetricsAction] Name: %s - previous query has invalid status skipping", a.GetName(symtab, logger)))
		return nil
	}

	if r_val, ok := symtab["__channel"]; ok {
		if metric_channel, ok = r_val.(chan<- Metric); !ok {
			panic(fmt.Sprintf("collid=\"%s\" script=\"%s\" msg=\"invalid context (channel wrong type)\"",
				CollectorId(symtab, logger),
				ScriptName(symtab, logger)))
		}
	} else {
		panic(fmt.Sprintf("collid=\"%s\" script=\"%s\" msg=\"invalid context (channel not set)\"",
			CollectorId(symtab, logger),
			ScriptName(symtab, logger)))
	}

	// check if user has specified a
	tmp_symtab := symtab
	// check if user has specified a scope for result : change the symtab access to that scope
	if a.Scope != "" && a.Scope != "none" {
		var err error
		tmp_symtab, err = SetScope(a.Scope, tmp_symtab)
		if err != nil {
			level.Warn(logger).Log(
				"collid", CollectorId(symtab, logger),
				"script", ScriptName(symtab, logger),
				"errmsg", err)
		}

		tmp_symtab["__collector_id"] = symtab["__collector_id"]
		tmp_symtab["__name__"] = symtab["__name__"]
	}
	for _, cur_act := range a.Actions {
		tmp_symtab["__channel"] = metric_channel
		// tmp_symtab["__metricfamilies"] = mfs
		// fmt.Printf("\tadd to symbols table: %s = %v\n", key, val)
		if err := PlayBaseAction(script, tmp_symtab, logger, cur_act, cur_act.CustomAction); err != nil {
			return err
		}

	}
	if a.Scope != "" && a.Scope != "none" && tmp_symtab != nil {
		delete(tmp_symtab, "__name__")
		delete(tmp_symtab, "__collector_id")
	}

	return nil
}

// ***************************************************************************************
func (a *MetricsAction) AddCustomTemplate(customTemplate *exporterTemplate) error {

	if err := AddCustomTemplate(a, customTemplate); err != nil {
		return err
	}

	for _, cur_act := range a.Actions {
		err := cur_act.AddCustomTemplate(customTemplate)
		if err != nil {
			return err
		}
	}

	return nil
}

// ***************************************************************************************
