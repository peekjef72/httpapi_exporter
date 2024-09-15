package main

import (
	"fmt"
	"log/slog"
)

// ***************************************************************************************
// ***************************************************************************************
// play_script_fact
// ***************************************************************************************
// ***************************************************************************************

type PlayScriptAction struct {
	Name                 *Field              `yaml:"name,omitempty" json:"name,omitempty"`
	With                 []any               `yaml:"with,omitempty" json:"with,omitempty"`
	When                 []*exporterTemplate `yaml:"when,omitempty" json:"when,omitempty"`
	LoopVar              string              `yaml:"loop_var,omitempty" json:"loop_var,omitempty"`
	Vars                 [][]any             `yaml:"vars,omitempty" json:"vars,omitempty"`
	Until                []*exporterTemplate `yaml:"until,omitempty" json:"until,omitempty"`
	PlayScriptActionName string              `yaml:"play_script" json:"play_script"`

	playScriptAction *YAMLScript
	vars             [][]any

	// Catches all undefined fields and must be empty after parsing.
	XXX map[string]interface{} `yaml:",inline" json:"-"`
}

func (a *PlayScriptAction) Type() int {
	return play_script_action
}

func (a *PlayScriptAction) GetName(symtab map[string]any, logger *slog.Logger) string {
	str, err := a.Name.GetValueString(symtab, nil, false)
	if err != nil {
		logger.Warn(
			fmt.Sprintf("invalid action name: %v", err),
			"collid", CollectorId(symtab, logger),
			"script", ScriptName(symtab, logger))
		return ""
	}
	return str
}

func (a *PlayScriptAction) GetNameField() *Field {
	return a.Name
}
func (a *PlayScriptAction) SetNameField(name *Field) {
	a.Name = name
}

func (a *PlayScriptAction) GetWidh() []any {
	return a.With
}
func (a *PlayScriptAction) SetWidth(with []any) {
	a.With = with
}

func (a *PlayScriptAction) GetWhen() []*exporterTemplate {
	return a.When

}
func (a *PlayScriptAction) SetWhen(when []*exporterTemplate) {
	a.When = when
}

func (a *PlayScriptAction) GetLoopVar() string {
	return a.LoopVar
}
func (a *PlayScriptAction) SetLoopVar(loopvar string) {
	a.LoopVar = loopvar
}

func (a *PlayScriptAction) GetVars() [][]any {
	return a.Vars
}
func (a *PlayScriptAction) SetVars(vars [][]any) {
	a.vars = vars
}

func (a *PlayScriptAction) GetUntil() []*exporterTemplate {
	return a.Until
}
func (a *PlayScriptAction) SetUntil(until []*exporterTemplate) {
	a.Until = until
}

func (a *PlayScriptAction) setBasicElement(
	nameField *Field,
	vars [][]any,
	with []any,
	loopVar string,
	when []*exporterTemplate,
	until []*exporterTemplate) error {
	return setBasicElement(a, nameField, vars, with, loopVar, when, until)
}

func (a *PlayScriptAction) PlayAction(script *YAMLScript, symtab map[string]any, logger *slog.Logger) error {
	return PlayBaseAction(script, symtab, logger, a, a.CustomAction)
}

// only for MetricsAction
func (a *PlayScriptAction) GetMetrics() []*GetMetricsRes {
	return nil
}

// only for MetricAction
func (a *PlayScriptAction) GetMetric() *MetricConfig {
	return nil
}
func (a *PlayScriptAction) SetMetricFamily(*MetricFamily) {
}

// only for PlayAction
func (a *PlayScriptAction) SetPlayAction(scripts map[string]*YAMLScript) error {
	if script, ok := scripts[a.PlayScriptActionName]; ok && script != nil {
		a.playScriptAction = script
		return nil
	}
	return fmt.Errorf("scriptname not found in play_script action %s", a.PlayScriptActionName)
}

// specific behavior for the PlayScriptAction
func (a *PlayScriptAction) CustomAction(script *YAMLScript, symtab map[string]any, logger *slog.Logger) error {
	logger.Debug(
		fmt.Sprintf("[Type: PlayScriptAction] Name: %s", a.GetName(symtab, logger)),
		"collid", CollectorId(symtab, logger),
		"script", ScriptName(symtab, logger),
		"name", a.GetName(symtab, logger))

	return a.playScriptAction.Play(symtab, false, logger)
}

func (a *PlayScriptAction) AddCustomTemplate(customTemplate *exporterTemplate) error {

	if err := AddCustomTemplate(a, customTemplate); err != nil {
		return err
	}
	return a.playScriptAction.AddCustomTemplate(customTemplate)
}

// ***************************************************************************************
