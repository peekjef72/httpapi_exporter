package main

import (
	//"bytes"
	"fmt"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
)

// ***************************************************************************************
// ***************************************************************************************
// set_fact
// ***************************************************************************************
// ***************************************************************************************

type SetFactAction struct {
	Name    *Field              `yaml:"name,omitempty"`
	With    []any               `yaml:"with,omitempty"`
	When    []*exporterTemplate `yaml:"when,omitempty"`
	LoopVar string              `yaml:"loop_var,omitempty"`
	Vars    map[string]any      `yaml:"vars,omitempty"`
	Until   []*exporterTemplate `yaml:"until,omitempty"`

	SetFact map[string]any `yaml:"set_fact"`

	setFact [][]any
}

func (a *SetFactAction) Type() int {
	return set_fact_action
}

func (a *SetFactAction) GetName(symtab map[string]any, logger log.Logger) string {
	str, err := a.Name.GetValueString(symtab, nil, false)
	if err != nil {
		level.Warn(logger).Log("msg", fmt.Sprintf("invalid action name: %v", err))
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

func (a *SetFactAction) GetVars() map[string]any {
	return a.Vars
}
func (a *SetFactAction) SetVars(vars map[string]any) {
	a.Vars = vars
}
func (a *SetFactAction) GetUntil() []*exporterTemplate {
	return a.Until
}
func (a *SetFactAction) SetUntil(until []*exporterTemplate) {
	a.Until = until
}

func (a *SetFactAction) setBasicElement(
	nameField *Field,
	vars map[string]any,
	with []any,
	loopVar string,
	when []*exporterTemplate,
	until []*exporterTemplate) error {
	return setBasicElement(a, nameField, vars, with, loopVar, when, until)
}

func (a *SetFactAction) PlayAction(script *YAMLScript, symtab map[string]any, logger log.Logger) error {

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

// specific behavior for the DebugAction
func (a *SetFactAction) CustomAction(script *YAMLScript, symtab map[string]any, logger log.Logger) error {
	var key_name string
	var err error
	var value_name any
	level.Debug(logger).Log(
		"script", ScriptName(symtab, logger),
		"msg", fmt.Sprintf("[Type: SetFactAction] Name: %s", Name(a.Name, symtab, logger)))
	for _, pair := range a.setFact {
		// tmp_map := map[string]any{}
		if pair == nil {
			return fmt.Errorf("set_fact: invalid key value")
		}
		if key, ok := pair[0].(*Field); ok {
			key_name, err = key.GetValueString(symtab, nil, false)
			if err == nil {
				if value_name, err = getValue(symtab, pair[1]); err != nil {
					return err
				}
				// switch value := pair[1].(type) {
				// case *Field:
				// 	if value_name, err = value.GetValueString(symtab, nil, false); err != nil {
				// 		return err
				// 	}
				// default:
				// 	// need to call a func to obtain a value from any type but with all content valorized
				// 	// => list: try to valorize each element
				// 	// => map: try to valorize key and value
				// 	value_name = value
				// }
				if value_name == nil {
					level.Debug(logger).Log(
						"script", ScriptName(symtab, logger),
						"msg", fmt.Sprintf("    remove from symbols table: %s", key_name))
					delete(symtab, key_name)
				} else {
					if key_name != "_" {
						level.Debug(logger).Log(
							"script", ScriptName(symtab, logger),
							"msg", fmt.Sprintf("    add to symbols table: %s = '%v'", key_name, value_name))
						symtab[key_name] = value_name
					} else {
						level.Debug(logger).Log(
							"script", ScriptName(symtab, logger),
							"msg", "    result discard (key >'_')")

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
					return fmt.Errorf("error in set_fact value: %s", err)
				}
			}
		}
	}

	return nil
}

// ***************************************************************************************
// ***************************************************************************************

func getMapValue(symtab map[string]any, raw_maps map[any]any) (map[any]any, error) {
	var err error
	final_res := make(map[any]any)
	for raw_key, raw_value := range raw_maps {
		key, err := getValue(symtab, raw_key)
		if err != nil {
			return nil, fmt.Errorf("invalid template for var key %s: %s", raw_key, err)
		}
		value, err := getValue(symtab, raw_value)
		if err != nil {
			return nil, fmt.Errorf("invalid template for var value %s: %s", raw_value, err)
		}
		final_res[key] = value
	}
	return final_res, err
}

func getSliceValue(symtab map[string]any, raw_slice []any) (any, error) {
	var err error
	final_res := make([]any, len(raw_slice))
	for idx, r_value := range raw_slice {
		res, err := getValue(symtab, r_value)
		if err != nil {
			return nil, fmt.Errorf("invalid template for var key %q: %s", res, err)
		}
		final_res[idx] = res

	}
	return final_res, err
}

func getValue(symtab map[string]any, raw_value any) (value_name any, err error) {
	switch value := raw_value.(type) {
	case *Field:
		if value_name, err = value.GetValueString(symtab, nil, false); err != nil {
			return nil, err
		}
	case []any:
		if value_name, err = getSliceValue(symtab, value); err != nil {
			return nil, err
		}
	case map[any]any:
		if value_name, err = getMapValue(symtab, value); err != nil {
			return nil, err
		}
	default:
		// need to call a func to obtain a value from any type but with all content valorized
		// => list: try to valorize each element
		// => map: try to valorize key and value
		value_name = value
	}
	return value_name, err
}

// ***************************************************************************************
