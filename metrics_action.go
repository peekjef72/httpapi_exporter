// cSpell:ignore maprefix, elmt
package main

import (
	//"bytes"
	"fmt"
	"log/slog"
	"strings"
)

// ***************************************************************************************
// ***************************************************************************************
// metrics action (block / loop )
// ***************************************************************************************
// ***************************************************************************************

type MetricsAction struct {
	// BaseAction
	Name    *Field         `yaml:"name,omitempty" json:"name,omitempty"`
	With    []any          `yaml:"with,omitempty" json:"with,omitempty"`
	When    []*Field       `yaml:"when,omitempty" json:"when,omitempty"`
	LoopVar string         `yaml:"loop_var,omitempty" json:"loop_var,omitempty"`
	Vars    map[string]any `yaml:"vars,omitempty" json:"vars,omitempty"`
	Until   []*Field       `yaml:"until,omitempty" json:"until,omitempty"`

	Metrics      []*MetricConfig `yaml:"metrics" json:"metrics"`                                 // metrics defined by this collector
	Scope        string          `yaml:"scope,omitempty" json:"scope,omitempty"`                 // var path where to collect data: shortcut for {{ .scope.path.var }}
	MetricPrefix string          `yaml:"metric_prefix,omitempty" json:"metric_prefix,omitempty"` // var to alert metric name
	Actions      []Action        `yaml:"-" json:"-"`

	// Catches all undefined fields and must be empty after parsing.
	XXX map[string]interface{} `yaml:",inline" json:"-"`

	vars [][]any
}

func (a *MetricsAction) Type() int {
	return metrics_action
}

func (a *MetricsAction) TypeName() string {
	return "metrics_action"
}

func (a *MetricsAction) GetName(symtab map[string]any, logger *slog.Logger) string {
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

func (a *MetricsAction) GetNameField() *Field {
	return a.Name
}
func (a *MetricsAction) SetNameField(name *Field) {
	a.Name = name
}

func (a *MetricsAction) GetWidth() []any {
	return a.With
}
func (a *MetricsAction) SetWidth(with []any) {
	a.With = with
}

func (a *MetricsAction) GetWhen() []*Field {
	return a.When

}
func (a *MetricsAction) SetWhen(when []*Field) {
	a.When = when
}

func (a *MetricsAction) GetLoopVar() string {
	return a.LoopVar
}
func (a *MetricsAction) SetLoopVar(loopvar string) {
	a.LoopVar = loopvar
}

func (a *MetricsAction) GetVars() [][]any {
	return a.vars
}
func (a *MetricsAction) SetVars(vars [][]any) {
	a.vars = vars
}

func (a *MetricsAction) GetUntil() []*Field {
	return a.Until
}
func (a *MetricsAction) SetUntil(until []*Field) {
	a.Until = until
}

func (a *MetricsAction) setBasicElement(
	nameField *Field,
	vars [][]any,
	with []any,
	loopVar string,
	when []*Field,
	until []*Field) error {
	return setBasicElement(a, nameField, vars, with, loopVar, when, until)
}

func (a *MetricsAction) PlayAction(script *YAMLScript, symtab map[string]any, logger *slog.Logger) error {
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

const (
	error_scope_not_found = iota
	error_scope_invalid_type
)

type scopeError struct {
	code    int
	message string
}

type ScopeError interface {
	Code() int
	Error() string
}

func newScopeError(code int, msg string) *scopeError {
	return &scopeError{
		code:    code,
		message: msg,
	}
}

func (e *scopeError) Error() string {
	return fmt.Sprintf("SetScopeError %d: %s", e.code, e.message)
}

func (e *scopeError) Code() int {
	return e.code
}

// ***************************************************************************************
// specific behavior for the MetricsAction
func SetScope(scope string, symtab map[string]any) (map[string]any, error) {
	var err error

	tmp_symtab := symtab
	// remove first char if it is . like for gotemplate var .var.name.
	// or $ like for path variable: $usage.data
	if scope[0] == '.' || scope[0] == '$' {
		scope = scope[1:]
	}
	// split the scope string into parts: attr1.attr[0].attr
	vars := strings.Split(scope, ".")
	// last_elmt := len(vars) -1
	for _, var_name := range vars {
		if raw_value, ok := tmp_symtab[var_name]; ok {
			switch cur_value := raw_value.(type) {
			case map[string]any:
				tmp_symtab = cur_value
			default:
				err = newScopeError(error_scope_invalid_type, fmt.Sprintf("can't set scope: identifier '%s' has invalid type", var_name))
			}
			// }
		} else {
			err = newScopeError(error_scope_invalid_type, fmt.Sprintf("can't set scope: identifier '%s' not found", var_name))
		}
	}
	return tmp_symtab, err
}

// ***************************************************************************************
func (a *MetricsAction) CustomAction(script *YAMLScript, symtab map[string]any, logger *slog.Logger) error {

	var (
		metric_channel chan<- Metric
	)

	logger.Debug(
		fmt.Sprintf("[Type: MetricsAction] Name: %s - %d metrics_name to set", a.GetName(symtab, logger), len(a.Metrics)),
		"coll", CollectorId(symtab, logger),
		"script", ScriptName(symtab, logger),
	)

	// this can't arrive because previous c.Collect() / c.client.Execute() has returned ErrInvalidQueryResult
	// so collect() stops and don't play metrics_actions.
	query_status, ok := GetMapValueBool(symtab, "query_status")
	if !ok || (ok && !query_status) {
		logger.Debug(
			fmt.Sprintf("[Type: MetricsAction] Name: %s - previous query has invalid status skipping", a.GetName(symtab, logger)),
			"coll", CollectorId(symtab, logger),
			"script", ScriptName(symtab, logger),
			"name", a.GetName(symtab, logger))
		return ErrInvalidQueryResult
	}

	if r_val, ok := symtab["__metric_channel"]; ok {
		if metric_channel, ok = r_val.(chan<- Metric); !ok {
			panic(fmt.Sprintf("coll=\"%s\" script=\"%s\" name=\"%s\" msg=\"invalid context (metric channel wrong type)\"",
				CollectorId(symtab, logger),
				ScriptName(symtab, logger),
				a.GetName(symtab, logger)))
		}
	} else {
		panic(fmt.Sprintf("coll=\"%s\" script=\"%s\" name=\"%s\" msg=\"invalid context (metric channel not set)\"",
			CollectorId(symtab, logger),
			ScriptName(symtab, logger),
			a.GetName(symtab, logger)))
	}

	// check if user has specified a
	tmp_symtab := symtab
	// check if user has specified a scope for result : change the symtab access to that scope
	if a.Scope != "" && a.Scope != "none" {
		var err error
		tmp_symtab, err = SetScope(a.Scope, tmp_symtab)
		if err != nil {
			logger.Warn(
				err.Error(),
				"coll", CollectorId(symtab, logger),
				"script", ScriptName(symtab, logger),
				"name", a.GetName(symtab, logger))
		}

		tmp_symtab["__collector_id"] = symtab["__collector_id"]
		tmp_symtab["__name__"] = symtab["__name__"]
		tmp_symtab["root"] = symtab
		defer func() {
			delete(tmp_symtab, "__name__")
			delete(tmp_symtab, "__collector_id")
			delete(tmp_symtab, "root")
			// logger.Debug(
			// 	"[Type: MetricsAction] remove root from symtab",
			// 	"coll", CollectorId(symtab, logger),
			// 	"script", ScriptName(symtab, logger),
			// 	"name", a.GetName(symtab, logger))
		}()
	}
	for _, cur_act := range a.Actions {
		tmp_symtab["__metric_channel"] = metric_channel
		if err := PlayBaseAction(script, tmp_symtab, logger, cur_act, cur_act.CustomAction); err != nil {
			return err
		}

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
