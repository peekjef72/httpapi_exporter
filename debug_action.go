package main

import (
	//"bytes"
	"fmt"
	"log/slog"
)

// ***************************************************************************************
// ***************************************************************************************
// debug action
// ***************************************************************************************
// ***************************************************************************************

// ****************************

type DebugActionConfig struct {
	MsgVal string `yaml:"msg" json:"msg"`

	msg *Field

	// Catches all undefined fields and must be empty after parsing.
	XXX map[string]interface{} `yaml:",inline" json:"-"`
}

// UnmarshalYAML implements the yaml.Unmarshaler interface for DebugActionConfig.
func (dc *DebugActionConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type plain DebugActionConfig
	var err error
	if err := unmarshal((*plain)(dc)); err != nil {
		return err
	}
	// Check required fields
	dc.msg, err = NewField(dc.MsgVal, nil)
	if err != nil {
		return fmt.Errorf("invalid template for debug message %q: %s", dc.MsgVal, err)
	}

	return checkOverflow(dc.XXX, "debug action")
}

// ****************************
type DebugAction struct {
	Name    *Field              `yaml:"name,omitempty" json:"name,omitempty"`
	With    []any               `yaml:"with,omitempty" json:"with,omitempty"`
	When    []*exporterTemplate `yaml:"when,omitempty" json:"when,omitempty"`
	LoopVar string              `yaml:"loop_var,omitempty" json:"loop_var,omitempty"`
	Vars    map[string]any      `yaml:"vars,omitempty" json:"vars,omitempty"`
	Until   []*exporterTemplate `yaml:"until,omitempty" json:"until,omitempty"`

	Debug *DebugActionConfig `yaml:"debug" json:"debug"`
	vars  [][]any
}

func (a *DebugAction) Type() int {
	return debug_action
}

func (a *DebugAction) GetName(symtab map[string]any, logger *slog.Logger) string {
	str, err := a.Name.GetValueString(symtab)
	if err != nil {
		logger.Warn(
			fmt.Sprintf("invalid action name: %v", err),
			"collid", CollectorId(symtab, logger),
			"script", ScriptName(symtab, logger))
		return ""
	}
	return str
}

func (a *DebugAction) GetNameField() *Field {
	return a.Name
}
func (a *DebugAction) SetNameField(name *Field) {
	a.Name = name
}

func (a *DebugAction) GetWidh() []any {
	return a.With
}
func (a *DebugAction) SetWidth(with []any) {
	a.With = with
}

func (a *DebugAction) GetWhen() []*exporterTemplate {
	return a.When

}
func (a *DebugAction) SetWhen(when []*exporterTemplate) {
	a.When = when
}

func (a *DebugAction) GetLoopVar() string {
	return a.LoopVar
}
func (a *DebugAction) SetLoopVar(loopvar string) {
	a.LoopVar = loopvar
}

func (a *DebugAction) GetVars() [][]any {
	return a.vars
}
func (a *DebugAction) SetVars(vars [][]any) {
	a.vars = vars
}

func (a *DebugAction) GetUntil() []*exporterTemplate {
	return a.Until
}
func (a *DebugAction) SetUntil(until []*exporterTemplate) {
	a.Until = until
}

// func (a *DebugAction) GetBaseAction() *BaseAction {
// 	return nil
// }

func (a *DebugAction) setBasicElement(
	nameField *Field,
	vars [][]any,
	with []any,
	loopVar string,
	when []*exporterTemplate,
	until []*exporterTemplate) error {
	return setBasicElement(a, nameField, vars, with, loopVar, when, until)
}

func (a *DebugAction) PlayAction(script *YAMLScript, symtab map[string]any, logger *slog.Logger) error {
	return PlayBaseAction(script, symtab, logger, a, a.CustomAction)
}

// only for MetricsAction
func (a *DebugAction) GetMetrics() []*GetMetricsRes {
	return nil
}

// only for MetricAction
func (a *DebugAction) GetMetric() *MetricConfig {
	return nil
}
func (a *DebugAction) SetMetricFamily(*MetricFamily) {
}

// only for PlayAction
func (a *DebugAction) SetPlayAction(scripts map[string]*YAMLScript) error {
	return nil
}

// specific behavior for the DebugAction
func (a *DebugAction) CustomAction(script *YAMLScript, symtab map[string]any, logger *slog.Logger) error {
	logger.Debug(
		"[Type: DebugAction]",
		"collid", CollectorId(symtab, logger),
		"script", ScriptName(symtab, logger),
		"name", a.GetName(symtab, logger))

	str, err := a.Debug.msg.GetValueString(symtab)
	if err != nil {
		str = a.Debug.MsgVal
		logger.Warn(
			fmt.Sprintf("invalid template for debug message '%s': %v", str, err),
			"collid", CollectorId(symtab, logger),
			"script", ScriptName(symtab, logger),
			"name", a.GetName(symtab, logger))
	}

	logger.Debug(
		fmt.Sprintf("    message: %s", str),
		"collid", CollectorId(symtab, logger),
		"script", ScriptName(symtab, logger),
		"name", a.GetName(symtab, logger))

	return nil
}

func (a *DebugAction) AddCustomTemplate(customTemplate *exporterTemplate) error {

	if err := AddCustomTemplate(a, customTemplate); err != nil {
		return err
	}
	if a.Debug.msg != nil {
		if err := a.Debug.msg.AddDefaultTemplate(customTemplate); err != nil {
			return err
		}
	}

	return nil
}

// ***************************************************************************************
