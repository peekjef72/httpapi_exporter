package main

import (
	//"bytes"
	"fmt"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
)

// ***************************************************************************************
// ***************************************************************************************
// actions (block / loop )
// ***************************************************************************************
// ***************************************************************************************

type ActionsAction struct {
	Name    *Field              `yaml:"name,omitempty"`
	With    []any               `yaml:"with,omitempty"`
	When    []*exporterTemplate `yaml:"when,omitempty"`
	LoopVar string              `yaml:"loop_var,omitempty"`
	Vars    map[string]any      `yaml:"vars,omitempty"`
	Until   []*exporterTemplate `yaml:"until,omitempty"`

	Actions []Action `yaml:"actions"`

	// Catches all undefined fields and must be empty after parsing.
	XXX map[string]interface{} `yaml:",inline" json:"-"`
}

func (a *ActionsAction) Type() int {
	return actions_action
}

func (a *ActionsAction) GetName(symtab map[string]any, logger log.Logger) string {
	str, err := a.Name.GetValueString(symtab, nil, false)
	if err != nil {
		level.Warn(logger).Log("msg", fmt.Sprintf("invalid action name: %v", err))
		return ""
	}
	return str
}

func (a *ActionsAction) GetNameField() *Field {
	return a.Name
}
func (a *ActionsAction) SetNameField(name *Field) {
	a.Name = name
}

func (a *ActionsAction) GetWidh() []any {
	return a.With
}
func (a *ActionsAction) SetWidth(with []any) {
	a.With = with
}

func (a *ActionsAction) GetWhen() []*exporterTemplate {
	return a.When

}
func (a *ActionsAction) SetWhen(when []*exporterTemplate) {
	a.When = when
}

func (a *ActionsAction) GetLoopVar() string {
	return a.LoopVar
}
func (a *ActionsAction) SetLoopVar(loopvar string) {
	a.LoopVar = loopvar
}

func (a *ActionsAction) GetVars() map[string]any {
	return a.Vars
}
func (a *ActionsAction) SetVars(vars map[string]any) {
	a.Vars = vars
}

func (a *ActionsAction) GetUntil() []*exporterTemplate {
	return a.Until
}
func (a *ActionsAction) SetUntil(until []*exporterTemplate) {
	a.Until = until
}

// func (a *ActionsAction) GetBaseAction() *BaseAction {
// 	return nil
// }

func (a *ActionsAction) setBasicElement(
	nameField *Field,
	vars map[string]any,
	with []any,
	loopVar string,
	when []*exporterTemplate,
	until []*exporterTemplate) error {
	return setBasicElement(a, nameField, vars, with, loopVar, when, until)
}

func (a *ActionsAction) PlayAction(script *YAMLScript, symtab map[string]any, logger log.Logger) error {
	return PlayBaseAction(script, symtab, logger, a, a.CustomAction)
}

// WARNING: only the first MetricPrefix in actionsTree if supported
func (a *ActionsAction) GetMetrics() []*GetMetricsRes {
	var (
		final_res []*GetMetricsRes
	)
	for _, cur_act := range a.Actions {
		res := cur_act.GetMetrics()
		if len(res) > 0 {
			final_res = append(final_res, res...)
		}
	}
	return final_res
}

// only for MetricAction
func (a *ActionsAction) GetMetric() *MetricConfig {
	return nil
}
func (a *ActionsAction) SetMetricFamily(*MetricFamily) {
}

// only for PlayAction
func (a *ActionsAction) SetPlayAction(script map[string]*YAMLScript) error {
	for _, a := range a.Actions {
		if a.Type() == play_script_action || a.Type() == actions_action {
			if err := a.SetPlayAction(script); err != nil {
				return err
			}
		}
	}
	return nil
}

// specific behavior for the ActionsAction
func (a *ActionsAction) CustomAction(script *YAMLScript, symtab map[string]any, logger log.Logger) error {
	level.Debug(logger).Log(
		"script", ScriptName(symtab, logger),
		"msg", fmt.Sprintf("[Type: ActionsAction] Name: %s - %d Actions to play", a.GetName(symtab, logger), len(a.Actions)))
	for _, cur_act := range a.Actions {
		// fmt.Printf("\tadd to symbols table: %s = %v\n", key, val)
		if err := PlayBaseAction(script, symtab, logger, cur_act, cur_act.CustomAction); err != nil {
			return err
		}

	}
	return nil
}

func (a *ActionsAction) AddCustomTemplate(customTemplate *exporterTemplate) error {

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
