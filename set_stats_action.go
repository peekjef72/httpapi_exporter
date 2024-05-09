package main

import (
	"fmt"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
)

// ***************************************************************************************
// ***************************************************************************************
// set_fact
// ***************************************************************************************
// ***************************************************************************************

type SetStatsAction struct {
	Name    *Field              `yaml:"name,omitempty"`
	With    []any               `yaml:"with,omitempty"`
	When    []*exporterTemplate `yaml:"when,omitempty"`
	LoopVar string              `yaml:"loop_var,omitempty"`
	Vars    [][]any             `yaml:"vars,omitempty"`
	Until   []*exporterTemplate `yaml:"until,omitempty"`

	SetStats map[string]any `yaml:"set_stats"`

	setStats [][]any
	vars     [][]any
}

func (a *SetStatsAction) Type() int {
	return set_fact_action
}

func (a *SetStatsAction) GetName(symtab map[string]any, logger log.Logger) string {
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

func (a *SetStatsAction) GetNameField() *Field {
	return a.Name
}
func (a *SetStatsAction) SetNameField(name *Field) {
	a.Name = name
}

func (a *SetStatsAction) GetWidh() []any {
	return a.With
}
func (a *SetStatsAction) SetWidth(with []any) {
	a.With = with
}

func (a *SetStatsAction) GetWhen() []*exporterTemplate {
	return a.When

}
func (a *SetStatsAction) SetWhen(when []*exporterTemplate) {
	a.When = when
}

func (a *SetStatsAction) GetLoopVar() string {
	return a.LoopVar
}
func (a *SetStatsAction) SetLoopVar(loopvar string) {
	a.LoopVar = loopvar
}

func (a *SetStatsAction) GetVars() [][]any {
	return a.vars
}
func (a *SetStatsAction) SetVars(vars [][]any) {
	a.vars = vars
}
func (a *SetStatsAction) GetUntil() []*exporterTemplate {
	return a.Until
}
func (a *SetStatsAction) SetUntil(until []*exporterTemplate) {
	a.Until = until
}

func (a *SetStatsAction) setBasicElement(
	nameField *Field,
	vars [][]any,
	with []any,
	loopVar string,
	when []*exporterTemplate,
	until []*exporterTemplate) error {
	return setBasicElement(a, nameField, vars, with, loopVar, when, until)
}

func (a *SetStatsAction) PlayAction(script *YAMLScript, symtab map[string]any, logger log.Logger) error {

	return PlayBaseAction(script, symtab, logger, a, a.CustomAction)
}

// only for MetricsAction
func (a *SetStatsAction) GetMetrics() []*GetMetricsRes {
	return nil
}

// only for MetricAction
func (a *SetStatsAction) GetMetric() *MetricConfig {
	return nil
}
func (a *SetStatsAction) SetMetricFamily(*MetricFamily) {
}

// only for PlayAction
func (a *SetStatsAction) SetPlayAction(scripts map[string]*YAMLScript) error {
	return nil
}

// specific behavior for the SetStatsAction
func (a *SetStatsAction) CustomAction(script *YAMLScript, symtab map[string]any, logger log.Logger) error {

	var (
		key_name   string
		err        error
		value_name any
	)

	level.Debug(logger).Log(
		"collid", CollectorId(symtab, logger),
		"script", ScriptName(symtab, logger),
		"name", a.GetName(symtab, logger),
		"msg", "[Type: SetStatsAction]")

	dst_symtab := make(map[string]any)
	for _, pair := range a.setStats {
		if pair == nil {
			return fmt.Errorf("set_fact: invalid key value")
		}
		if key, ok := pair[0].(*Field); ok {
			key_name, err = key.GetValueString(symtab, nil, false)
			if err == nil {
				if value_name, err = ValorizeValue(symtab, pair[1], logger, a.GetName(symtab, logger), 0); err != nil {
					return err
				}
				if value_name == nil {
					level.Debug(logger).Log(
						"collid", CollectorId(symtab, logger),
						"script", ScriptName(symtab, logger),
						"name", a.GetName(symtab, logger),
						"msg", fmt.Sprintf("    %s is nil: not set into set_stats", key_name))
				} else {
					if value_name == "_" {
						// null op key_name :
					} else {
						if key_name != "_" {
							level.Debug(logger).Log(
								"collid", CollectorId(symtab, logger),
								"script", ScriptName(symtab, logger),
								"name", a.GetName(symtab, logger),
								"msg", fmt.Sprintf("    add to symbols table: %s = '%v'", key_name, value_name))
							if err := SetSymTab(dst_symtab, key_name, value_name); err != nil {
								level.Warn(logger).Log(
									"collid", CollectorId(symtab, logger),
									"script", ScriptName(symtab, logger),
									"name", a.GetName(symtab, logger),
									"msg", fmt.Sprintf("error setting map value for key '%s'", key_name), "errmsg", err)
								continue

							}
						} else {
							level.Debug(logger).Log(
								"collid", CollectorId(symtab, logger),
								"script", ScriptName(symtab, logger),
								"name", a.GetName(symtab, logger),
								"msg", "    result discard (key >'_')")
						}
					}
				}
			}
		}
	}
	if len(dst_symtab) > 0 {
		symtab["set_stats"] = dst_symtab
	}

	return nil
}

func (a *SetStatsAction) AddCustomTemplate(customTemplate *exporterTemplate) error {

	if err := AddCustomTemplate(a, customTemplate); err != nil {
		return err
	}

	for _, pair := range a.setStats {
		// tmp_map := map[string]any{}
		if pair == nil {
			return fmt.Errorf("set_fact: invalid key value")
		}
		if key, ok := pair[0].(*Field); ok {
			if key != nil {
				if err := key.AddDefaultTemplate(customTemplate); err != nil {
					return err
				}
			}
			if pair[1] != nil {
				if err := AddCustomTemplateElement(pair[1], customTemplate); err != nil {
					return fmt.Errorf("error in set_stats value: %s", err)
				}
			}
		}
	}

	return nil
}

// ***************************************************************************************
