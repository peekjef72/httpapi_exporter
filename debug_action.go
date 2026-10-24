package main

import (
	//"bytes"
	"fmt"
	"log/slog"

	"github.com/peekjef72/httpapi_exporter/goja_modules"
)

// ***************************************************************************************
// ***************************************************************************************
// debug action
// ***************************************************************************************
// ***************************************************************************************

// ****************************

type DebugActionConfig struct {
	MsgVal string `yaml:"msg" json:"msg"`
	Level  string `yaml:"level" json:"level"`

	registry   *goja_modules.JSRegistry
	msg, level *Field

	// Catches all undefined fields and must be empty after parsing.
	XXX map[string]any `yaml:",inline" json:"-"`
}

// UnmarshalYAML implements the yaml.Unmarshaler interface for DebugActionConfig.
func (dc *DebugActionConfig) UnmarshalYAML(unmarshal func(any) error) error {
	type plain DebugActionConfig
	var err error
	dc.Level = "debug"
	if err := unmarshal((*plain)(dc)); err != nil {
		return err
	}
	// Check required fields
	dc.msg, err = NewField(dc.MsgVal, nil, dc.registry)
	if err != nil {
		return fmt.Errorf("invalid template for debug message %q: %s", dc.MsgVal, err)
	}
	dc.level, err = NewField(dc.Level, nil, dc.registry)
	if err != nil {
		return fmt.Errorf("invalid template for debug level %q: %s", dc.Level, err)
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

func (a *DebugAction) GetWith() []any {
	return a.With
}
func (a *DebugAction) SetWith(with []any) {
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
	registry *goja_modules.JSRegistry,
	nameField *Field,
	vars [][]any,
	with []any,
	loopVar string,
	when []*Field,
	until []*Field) error {
	return setBasicElement(a, registry, nameField, vars, with, loopVar, when, until)
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
	var log func(msg string, args ...any)

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

	level, err := a.Debug.level.GetValueString(symtab, logger)
	if err != nil {
		str = a.Debug.Level
		logger.Warn(
			fmt.Sprintf("invalid template for debug level '%s': %v", str, err),
			"coll", CollectorId(symtab, logger),
			"script", ScriptName(symtab, logger),
			"name", a.GetName(symtab, logger))
		level = "debug"
	}
	switch level {
	case "debug":
		// Level = slog.LevelDebug
		log = logger.Debug
	case "info":
		// Level = slog.LevelInfo
		log = logger.Info
	case "warn":
		// Level = slog.LevelWarn
		log = logger.Warn
	case "error":
		// Level = slog.LevelError
		log = logger.Error
	default:
		logger.Error(fmt.Sprintf("invalid log.level specified %s. reset to debug", level))
		log = logger.Debug
	}
	log(
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
