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
	Name    *Field         `yaml:"name,omitempty" json:"name,omitempty"`
	With    []any          `yaml:"with,omitempty" json:"with,omitempty"`
	When    []*Field       `yaml:"when,omitempty" json:"when,omitempty"`
	LoopVar string         `yaml:"loop_var,omitempty" json:"loop_var,omitempty"`
	Vars    map[string]any `yaml:"vars,omitempty" json:"vars,omitempty"`
	Until   []*Field       `yaml:"until,omitempty" json:"until,omitempty"`

	Debug *DebugActionConfig `yaml:"debug" json:"debug"`
	vars  [][]any
}

func (a *DebugAction) Type() int {
	return debug_action
}

func (a *DebugAction) TypeName() string {
	return "debug_action"
}

func (a *DebugAction) GetName(symtab map[string]any, logger *slog.Logger) string {
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

func (a *DebugAction) GetNameField() *Field {
	return a.Name
}
func (a *DebugAction) SetNameField(name *Field) {
	a.Name = name
}

func (a *DebugAction) GetWidth() []any {
	return a.With
}
func (a *DebugAction) SetWidth(with []any) {
	a.With = with
}

func (a *DebugAction) GetWhen() []*Field {
	return a.When

}
func (a *DebugAction) SetWhen(when []*Field) {
	a.When = when
}

func (a *DebugAction) GetLoopVar() string {
	return a.LoopVar
}
func (a *DebugAction) SetLoopVar(loopVar string) {
	a.LoopVar = loopVar
}

func (a *DebugAction) GetVars() [][]any {
	return a.vars
}
func (a *DebugAction) SetVars(vars [][]any) {
	a.vars = vars
}

func (a *DebugAction) GetUntil() []*Field {
	return a.Until
}
func (a *DebugAction) SetUntil(until []*Field) {
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
	when []*Field,
	until []*Field) error {
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
		"coll", CollectorId(symtab, logger),
		"script", ScriptName(symtab, logger),
		"name", a.GetName(symtab, logger))

	str, err := a.Debug.msg.GetValueString(symtab, logger)
	if err != nil {
		str = a.Debug.MsgVal
		logger.Warn(
			fmt.Sprintf("invalid template for debug message '%s': %v", str, err),
			"coll", CollectorId(symtab, logger),
			"script", ScriptName(symtab, logger),
			"name", a.GetName(symtab, logger))
	}

	logger.Debug(
		fmt.Sprintf("    message: %s", str),
		"coll", CollectorId(symtab, logger),
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
