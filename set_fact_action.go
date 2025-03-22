package main

import (
	"errors"
	"fmt"
	"log/slog"
)

// ***************************************************************************************
// ***************************************************************************************
// set_fact
// ***************************************************************************************
// ***************************************************************************************

type SetFactAction struct {
	Name    *Field              `yaml:"name,omitempty" json:"name,omitempty"`
	With    []any               `yaml:"with,omitempty" json:"with,omitempty"`
	When    []*exporterTemplate `yaml:"when,omitempty" json:"when,omitempty"`
	LoopVar string              `yaml:"loop_var,omitempty" json:"loop_var,omitempty"`
	Vars    [][]any             `yaml:"vars,omitempty" json:"vars,omitempty"`
	Until   []*exporterTemplate `yaml:"until,omitempty" json:"until,omitempty"`

	SetFact map[string]any `yaml:"set_fact" json:"set_fact"`

	setFact [][]any
	vars    [][]any
}

func (a *SetFactAction) Type() int {
	return set_fact_action
}

func (a *SetFactAction) GetName(symtab map[string]any, logger *slog.Logger) string {
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

func (a *SetFactAction) GetNameField() *Field {
	return a.Name
}
func (a *SetFactAction) SetNameField(name *Field) {
	a.Name = name
}

func (a *SetFactAction) GetWidh() []any {
	return a.With
}
func (a *SetFactAction) SetWidth(with []any) {
	a.With = with
}

func (a *SetFactAction) GetWhen() []*exporterTemplate {
	return a.When

}
func (a *SetFactAction) SetWhen(when []*exporterTemplate) {
	a.When = when
}

func (a *SetFactAction) GetLoopVar() string {
	return a.LoopVar
}
func (a *SetFactAction) SetLoopVar(loopvar string) {
	a.LoopVar = loopvar
}

func (a *SetFactAction) GetVars() [][]any {
	return a.vars
}
func (a *SetFactAction) SetVars(vars [][]any) {
	a.vars = vars
}
func (a *SetFactAction) GetUntil() []*exporterTemplate {
	return a.Until
}
func (a *SetFactAction) SetUntil(until []*exporterTemplate) {
	a.Until = until
}

func (a *SetFactAction) setBasicElement(
	nameField *Field,
	vars [][]any,
	with []any,
	loopVar string,
	when []*exporterTemplate,
	until []*exporterTemplate) error {
	return setBasicElement(a, nameField, vars, with, loopVar, when, until)
}

func (a *SetFactAction) PlayAction(script *YAMLScript, symtab map[string]any, logger *slog.Logger) error {

	return PlayBaseAction(script, symtab, logger, a, a.CustomAction)
}

// only for MetricsAction
func (a *SetFactAction) GetMetrics() []*GetMetricsRes {
	return nil
}

// only for MetricAction
func (a *SetFactAction) GetMetric() *MetricConfig {
	return nil
}
func (a *SetFactAction) SetMetricFamily(*MetricFamily) {
}

// only for PlayAction
func (a *SetFactAction) SetPlayAction(scripts map[string]*YAMLScript) error {
	return nil
}

// specific behavior for the SetStatsAction
// it does nothing at all... it is only en entry point for calling target to collect
// vars from the collector and so to make them persistent accross calls
func (a *SetFactAction) CustomAction(script *YAMLScript, symtab map[string]any, logger *slog.Logger) error {

	var (
		key_name   string
		err        error
		value_name any
	)

	logger.Debug(
		"[Type: SetFactAction]",
		"collid", CollectorId(symtab, logger),
		"script", ScriptName(symtab, logger),
		"name", a.GetName(symtab, logger))

	for _, pair := range a.setFact {
		if pair == nil {
			return errors.New("set_fact: invalid key value")
		}
		if key, ok := pair[0].(*Field); ok {
			key_name, err = key.GetValueString(symtab, nil, false)
			if err == nil {
				if value_name, err = ValorizeValue(symtab, pair[1], logger, a.GetName(symtab, logger), false); err != nil {
					return err
				}

				if value_name == nil {
					logger.Debug(
						fmt.Sprintf("    remove from symbols table: %s", key_name),
						"collid", CollectorId(symtab, logger),
						"script", ScriptName(symtab, logger),
						"name", a.GetName(symtab, logger))
					DeleteSymtab(symtab, key_name)
				} else {
					if key_name != "_" {
						logger.Debug(
							fmt.Sprintf("    add to symbols table: %s = '%v'", key_name, value_name),
							"collid", CollectorId(symtab, logger),
							"script", ScriptName(symtab, logger),
							"name", a.GetName(symtab, logger))
						if err := SetSymTab(symtab, key_name, value_name); err != nil {
							logger.Warn(
								fmt.Sprintf("error setting map value for key '%s'", key_name), "errmsg", err,
								"collid", CollectorId(symtab, logger),
								"script", ScriptName(symtab, logger),
								"name", a.GetName(symtab, logger))
							continue
						}
					} else {
						logger.Debug(
							"    result discard (key >'_')",
							"collid", CollectorId(symtab, logger),
							"script", ScriptName(symtab, logger),
							"name", a.GetName(symtab, logger))

					}
				}
			}
		}
	}

	return nil
}

func (a *SetFactAction) AddCustomTemplate(customTemplate *exporterTemplate) error {

	if err := AddCustomTemplate(a, customTemplate); err != nil {
		return err
	}

	for _, pair := range a.setFact {
		if pair == nil {
			return errors.New("set_fact: invalid key value")
		}
		if key, ok := pair[0].(*Field); ok {
			if key != nil {
				if err := key.AddDefaultTemplate(customTemplate); err != nil {
					return err
				}
			}
			if pair[1] != nil {
				if err := AddCustomTemplateElement(pair[1], customTemplate); err != nil {
					return fmt.Errorf("error in set_fact value: %s", err)
				}
			}
		}
	}

	return nil
}

// ***************************************************************************************
