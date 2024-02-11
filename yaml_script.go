package main

import (
	//"bytes"
	"fmt"
	"reflect"
	"strings"
	"text/template"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"gopkg.in/yaml.v3"
)

type YAMLScript struct {
	// name           string `yaml:"name"`
	Actions    ActionsList `yaml:"actions"`
	UntilLimit int         `yaml:"until_limit"`

	name            string
	customTemplate  *template.Template
	metricsActions  []*MetricsAction
	setStatsActions []*SetStatsAction
}

type DumpYAMLScript struct {
	Actions    ActionsList `yaml:"actions"`
	UntilLimit int         `yaml:"until_limit"`
}

//******************************************************************
//**
//******************************************************************

const (
	base_action        = iota
	set_fact_action    = iota
	actions_action     = iota
	debug_action       = iota
	query_action       = iota
	metrics_action     = iota
	metric_action      = iota
	play_script_action = iota
	set_stats          = iota
)

type GetMetricsRes struct {
	mc       []*MetricConfig
	maprefix string
}

type Action interface {
	BaseAction
	//	GetName(symtab map[string]any, logger log.Logger) string
	Type() int
	// GetBaseAction() *BaseAction
	setBasicElement(nameField *Field, vars [][]any, with []any, loopVar string, when []*exporterTemplate, until []*exporterTemplate) error
	PlayAction(script *YAMLScript, symtab map[string]any, logger log.Logger) error
	CustomAction(script *YAMLScript, symtab map[string]any, logger log.Logger) error
	// to dump config
	// String() string

	// only for MetricsAction
	GetMetrics() []*GetMetricsRes
	// only for MetricAction
	GetMetric() *MetricConfig
	SetMetricFamily(*MetricFamily)
	// only for PlayAction
	SetPlayAction(scripts map[string]*YAMLScript) error
	AddCustomTemplate(customTemplate *exporterTemplate) error
}

type ActionsList []Action

func (sc *YAMLScript) Play(symtab map[string]any, ignore_errors bool, logger log.Logger) error {
	symtab["__name__"] = sc.name
	for _, ac := range sc.Actions {
		err := ac.PlayAction(sc, symtab, logger)
		if !ignore_errors && err != nil {
			return err
		}
	}
	return nil
}

func (sc *YAMLScript) AddCustomTemplate(customTemplate *exporterTemplate) error {
	for _, ac := range sc.Actions {
		err := ac.AddCustomTemplate(customTemplate)
		if err != nil {
			return err
		}
	}
	return nil
}
func CollectorId(symtab map[string]any, logger log.Logger) string {
	raw_id, ok := symtab["__collector_id"]
	if !ok {
		level.Warn(logger).Log("msg", "invalid collector id")
		return ""
	}
	str, ok := raw_id.(string)
	if !ok {
		level.Warn(logger).Log("msg", "invalid collector id")
		return ""

	}
	return str
}

func ScriptName(symtab map[string]any, logger log.Logger) string {
	raw_name, ok := symtab["__name__"]
	if !ok {
		level.Warn(logger).Log("msg", "invalid script name")
		return ""
	}
	str, ok := raw_name.(string)
	if !ok {
		level.Warn(logger).Log("msg", "invalid script name type")
		return ""

	}
	return str
}

func Name(name *Field, symtab map[string]any, logger log.Logger) string {
	str, err := name.GetValueString(symtab, nil, false)
	if err != nil {
		level.Warn(logger).Log("msg", fmt.Sprintf("invalid action name: %v", err))
		return ""
	}
	return str
}

type BaseAction interface {
	GetName(symtab map[string]any, logger log.Logger) string

	GetNameField() *Field
	SetNameField(*Field)
	GetWidh() []any
	SetWidth([]any)
	GetWhen() []*exporterTemplate
	SetWhen([]*exporterTemplate)
	GetLoopVar() string
	SetLoopVar(string)
	GetVars() [][]any
	SetVars([][]any)
	GetUntil() []*exporterTemplate
	SetUntil([]*exporterTemplate)
}

func setBasicElement(
	ba BaseAction,
	nameField *Field,
	vars [][]any,
	with []any,
	loopVar string,
	when []*exporterTemplate,
	until []*exporterTemplate) error {

	if nameField != nil {
		ba.SetNameField(nameField)
	}
	if len(vars) > 0 {
		ba.SetVars(vars)
	}
	if len(with) > 0 {
		// have to check every element in the slide
		// build a Field for each.
		baWith := make([]any, len(with))
		for idx, elmt := range with {
			switch curval := elmt.(type) {
			case string:
				tt := strings.LastIndex(curval, "}}")
				if tt != -1 {
					curval = curval[:tt] + " | toRawJson " + curval[tt:]
				}
				field, err := NewField(curval, nil)
				if err != nil {
					return err
				}
				baWith[idx] = field
			case map[any]any:
				baWith[idx] = elmt
			case int:
				tmp := fmt.Sprintf("%d", curval)
				field, err := NewField(tmp, nil)
				if err != nil {
					return err
				}
				baWith[idx] = field

			case *Field:
				baWith[idx] = curval

			default:
				return fmt.Errorf("with_items: invalid type: '%s'", reflect.TypeOf(elmt))
			}
		}
		ba.SetWidth(baWith)
		if loopVar != "" {
			ba.SetLoopVar(loopVar)
		}
		// ba.With = with
	}
	if len(when) > 0 {
		ba.SetWhen(when)
	}
	if len(until) > 0 {
		ba.SetUntil(until)
	}
	return nil
}

func AddCustomTemplateElement(item any, customTemplate *exporterTemplate) error {
	switch curval := item.(type) {
	case *Field:
		if curval != nil {
			if err := curval.AddDefaultTemplate(customTemplate); err != nil {
				return err
			}
		}
	case map[any]any:
		if curval == nil {
			return nil
		}
		for r_key, r_value := range curval {
			if err := AddCustomTemplateElement(r_key, customTemplate); err != nil {
				return err
			}
			if err := AddCustomTemplateElement(r_value, customTemplate); err != nil {
				return err
			}
		}
	case []any:
		if curval == nil {
			return nil
		}
		for _, r_value := range curval {
			if err := AddCustomTemplateElement(r_value, customTemplate); err != nil {
				return err
			}
		}
	default:
		// value is a constant 1 float or bool... nothing to do
	}
	return nil
}

func AddCustomTemplate(ba BaseAction, customTemplate *exporterTemplate) error {
	name := ba.GetNameField()
	if name != nil {
		if err := name.AddDefaultTemplate(customTemplate); err != nil {
			return err
		}
	}
	baWith := ba.GetWidh()
	for idx, with := range baWith {
		if err := AddCustomTemplateElement(with, customTemplate); err != nil {
			return fmt.Errorf("error in with[%d]: %s", idx, err)
		}
	}
	baUntil := ba.GetUntil()
	for idx, until_tmpl := range baUntil {
		if tmpl, err := AddDefaultTemplate(until_tmpl, customTemplate); err != nil {
			return fmt.Errorf("error in until[%d]: %s", idx, err)
		} else {
			baUntil[idx] = tmpl
		}
	}

	baWhen := ba.GetWhen()
	for idx, when_tmpl := range baWhen {
		if tmpl, err := AddDefaultTemplate(when_tmpl, customTemplate); err != nil {
			return fmt.Errorf("error in when[%d]: %s", idx, err)
		} else {
			baWhen[idx] = tmpl
		}
	}

	return nil
}

// ********************************************************************************
func ValorizeValue(symtab map[string]any, item any, logger log.Logger, action_name string, cur_level int) (any, error) {
	var data any
	var err error
	// fmt.Println("item = ", reflect.TypeOf(item))
	switch curval := item.(type) {
	case *Field:
		check_value := false
		if cur_level == 0 {
			// this is a template populate list with var from symtab
			data, err = curval.GetValueObject(symtab)
			if err != nil {
				return data, err
			}
			if data == nil {
				check_value = true
			} else if r_data, ok := data.([]any); ok {
				if r_data == nil {
					check_value = true
				}
			} else if s_data, ok := data.(string); ok {
				if s_data == "" {
					data = nil
				}
			}
			// if r_data, ok := data.([]any); ok {
			// 	if len(r_data) == 0 {
			// 		check_value = true
			// 	}
			// }
		} else {
			check_value = true
		}
		if check_value {
			data, err = curval.GetValueString(symtab, nil, false)
		}
		return data, err
	case map[any]any:
		ldata := make(map[string]any, len(curval))

		for r_key, r_value := range curval {
			key, err := ValorizeValue(symtab, r_key, logger, action_name, cur_level+1)
			if err != nil {
				level.Warn(logger).Log(
					"collid", CollectorId(symtab, logger),
					"script", ScriptName(symtab, logger),
					"name", action_name,
					"msg", fmt.Sprintf("error building map key: %v", r_key), "errmsg", err)
				continue
			}
			key_val := ""
			if r_key_val, ok := key.(string); ok {
				key_val = r_key_val
			} else {
				continue
			}
			value, err := ValorizeValue(symtab, r_value, logger, action_name, cur_level+1)
			if err != nil {
				level.Warn(logger).Log(
					"collid", CollectorId(symtab, logger),
					"script", ScriptName(symtab, logger),
					"name", action_name,
					"msg", fmt.Sprintf("error building map value for key '%s'", key_val), "errmsg", err)
				continue
			}
			if err := SetSymTab(ldata, key_val, value); err != nil {
				level.Warn(logger).Log(
					"collid", CollectorId(symtab, logger),
					"script", ScriptName(symtab, logger),
					"name", action_name,
					"msg", fmt.Sprintf("error setting map value for key '%s'", key_val), "errmsg", err)
				continue
			}
		}
		data = ldata
	case []any:
		ldata := make([]any, len(curval))
		for idx, r_value := range curval {
			values, err := ValorizeValue(symtab, r_value, logger, action_name, cur_level+1)
			if err != nil {
				level.Warn(logger).Log(
					"collid", CollectorId(symtab, logger),
					"script", ScriptName(symtab, logger),
					"name", action_name,
					"msg", fmt.Sprintf("error building list value for index: %d", idx), "errmsg", err)
				continue
			}
			ldata[idx] = values
		}
		data = ldata
	default:
		// do nothing on value: bool, int64, float64, string
		data = curval
		// level.Warn(logger).Log(
		// 	"collid", CollectorId(symtab, logger),
		// 	"script", ScriptName(symtab, logger),
		// 	"msg", fmt.Sprintf("invalid type for with_items: %s", item))
	}
	return data, nil
}

func DeleteSymtab(symtab map[string]any, key_name string) error {
	var err error
	tmp_symtab := symtab
	scope := key_name
	if scope[0] == '.' {
		scope = scope[1:]
	}
	vars := strings.Split(scope, ".")
	last_elmt := len(vars) - 1
	for i, var_name := range vars {
		if raw_value, ok := tmp_symtab[var_name]; ok {
			switch cur_value := raw_value.(type) {
			case map[string]any:
				if i == last_elmt {
					delete(tmp_symtab, var_name)
					break
				}
				tmp_symtab = cur_value
			default:
				if i != last_elmt {
					err = fmt.Errorf("can't set scope: '%s' has invalid type", var_name)
					break
				}
			}
		} else {
			break
		}
	}
	return err
}
func SetSymTab(symtab map[string]any, key_name string, value any) error {
	var err error
	tmp_symtab := symtab
	scope := key_name
	if scope[0] == '.' {
		scope = scope[1:]
	}
	vars := strings.Split(scope, ".")
	last_elmt := len(vars) - 1
	for i, var_name := range vars {
		if i == last_elmt {
			tmp_symtab[var_name] = value
			break
		}
		if raw_value, ok := tmp_symtab[var_name]; ok {
			switch cur_value := raw_value.(type) {
			case map[string]any:
				tmp_symtab = cur_value
			default:
				if i != last_elmt {
					err = fmt.Errorf("can't set scope: '%s' has invalid type", var_name)
					break
				}
			}
		} else {
			// key doesn't exist currently: add it
			cur_value := make(map[string]any)
			tmp_symtab[var_name] = cur_value
			tmp_symtab = cur_value
		}
	}
	return err
}

func preserve_sym_tab(symtab map[string]any, old_values map[string]any, key string, val any) error {
	if old_val, ok := symtab[key]; ok {
		if err := SetSymTab(old_values, key, old_val); err != nil {
			return err
		}
		// old_values[key] = old_val
	} else {
		if err := SetSymTab(old_values, key, "_"); err != nil {
			return err
		}
		// old_values[key] = "_"
	}
	if err := SetSymTab(symtab, key, val); err != nil {
		return err
	}
	// symtab[key] = val
	return nil
}

func evalCond(symtab map[string]any, condTpl *exporterTemplate) (cond bool, err error) {

	cond = false

	defer func() {
		// res and err are named out parameters, so if we set value for them in defer
		// set the returned values
		ok := false
		if r := recover(); r != nil {
			if err, ok = r.(error); !ok {
				err = fmt.Errorf("panic in evalCond template with undefined error")
			}
		}
	}()

	tmp_res := new(strings.Builder)

	err = ((*template.Template)(condTpl)).Execute(tmp_res, &symtab)
	if err != nil {
		return false, err
	}
	if tmp_res.String() == "true" {
		cond = true
	}
	return cond, nil
}

func PlayBaseAction(script *YAMLScript, symtab map[string]any, logger log.Logger, ba Action, customAction func(*YAMLScript, map[string]any, log.Logger) error) error {

	// to preverse values from symtab
	old_values := make(map[string]any)

	defer func() {
		for key, val := range old_values {
			if val == "_" {
				delete(symtab, key)
			} else {
				symtab[key] = val
			}
		}
	}()

	// add the local vars from the action to symbols table
	baVars := ba.GetVars()
	if len(baVars) > 0 {
		for _, pair := range baVars {
			if pair == nil {
				level.Warn(logger).Log(
					"collid", CollectorId(symtab, logger),
					"script", ScriptName(symtab, logger),
					"name", ba.GetName(symtab, logger),
					"msg", "invalid key value pair for vars")
				continue
			}
			if key, ok := pair[0].(*Field); ok {
				key_name, err := key.GetValueString(symtab, nil, false)
				if err == nil {
					value, err := ValorizeValue(symtab, pair[1], logger, ba.GetName(symtab, logger), 0)
					if err == nil {
						if value != nil {
							if s_value, ok := value.(string); ok {
								if s_value == "_" {
									DeleteSymtab(symtab, key_name)
									level.Debug(logger).Log(
										"collid", CollectorId(symtab, logger),
										"script", ScriptName(symtab, logger),
										"name", ba.GetName(symtab, logger),
										"msg", fmt.Sprintf("vars(%s) has '_' value (removed)", key_name))
								}
							} else {
								if err := preserve_sym_tab(symtab, old_values, key_name, value); err != nil {
									level.Warn(logger).Log(
										"collid", CollectorId(symtab, logger),
										"script", ScriptName(symtab, logger),
										"name", ba.GetName(symtab, logger),
										"msg", fmt.Sprintf("error preserve symtab (%s): %s", key_name, err))
								}
							}
						} else {
							DeleteSymtab(symtab, key_name)
							level.Debug(logger).Log(
								"collid", CollectorId(symtab, logger),
								"script", ScriptName(symtab, logger),
								"name", ba.GetName(symtab, logger),
								"msg", fmt.Sprintf("vars(%s) has nil value (removed)", key_name))
						}
					} else {
						level.Warn(logger).Log(
							"collid", CollectorId(symtab, logger),
							"script", ScriptName(symtab, logger),
							"name", ba.GetName(symtab, logger),
							"msg", fmt.Sprintf("no data found for vars(%s): %s", key, err))
					}
				} else {
					level.Warn(logger).Log(
						"collid", CollectorId(symtab, logger),
						"script", ScriptName(symtab, logger),
						"name", ba.GetName(symtab, logger),
						"msg", fmt.Sprintf("no data found for vars: %s", err))
				}
			}
		}
	}
	var items []any
	var loop_var string
	do_loop := false
	set_loops_var := false

	final_items := make([]any, 0)
	baWith := ba.GetWidh()
	if len(baWith) > 0 {
		// build a list of element from ba.With list of Field
		items = baWith
		for _, item := range items {

			data, err := ValorizeValue(symtab, item, logger, ba.GetName(symtab, logger), 0)
			if err == nil {
				if data != nil {
					if l_data, ok := data.([]any); ok {
						if len(l_data) > 0 {
							final_items = append(final_items, l_data...)
						} else {
							level.Warn(logger).Log(
								"collid", CollectorId(symtab, logger),
								"script", ScriptName(symtab, logger),
								"name", ba.GetName(symtab, logger),
								"msg", "with_items list empty.")
						}
					} else {
						if s_data, ok := data.(string); ok {
							if s_data != "null" {
								final_items = append(final_items, data)
							}
						} else {
							final_items = append(final_items, data)
						}
					}
				}
			} else {
				level.Warn(logger).Log(
					"collid", CollectorId(symtab, logger),
					"script", ScriptName(symtab, logger),
					"name", ba.GetName(symtab, logger),
					"msg", fmt.Sprintf("no data found for with_items: %s", err))
			}
		}
		items = final_items
		baLoopVar := ba.GetLoopVar()
		if baLoopVar != "" {
			loop_var = baLoopVar
		} else {
			loop_var = "item"
		}
		set_loops_var = true
	} else if len(ba.GetUntil()) > 0 {
		do_loop = true
	} else {
		items = make([]any, 1)
		items[0] = 0
	}

	if !do_loop {
		// loop on items
		for idx, item := range items {
			if set_loops_var {
				if err := preserve_sym_tab(symtab, old_values, loop_var, item); err != nil {
					level.Warn(logger).Log(
						"collid", CollectorId(symtab, logger),
						"script", ScriptName(symtab, logger),
						"name", ba.GetName(symtab, logger),
						"msg", fmt.Sprintf("error preserve symtab (%s): %s", loop_var, err))
				}
				preserve_sym_tab(symtab, old_values, "loop_var_idx", idx)
				preserve_sym_tab(symtab, old_values, "loop_var", loop_var)
			}
			// check if there are condition on the "item" loop;
			// if one is false break item the loop on next.
			baWhen := ba.GetWhen()
			if len(baWhen) > 0 {

				valid_value := true

				for i, condTpl := range baWhen {
					// tmp_res := new(strings.Builder)

					// err := ((*template.Template)(condTpl)).Execute(tmp_res, &symtab)
					cond, err := evalCond(symtab, condTpl)
					if err != nil {
						return fmt.Errorf("invalid template value for 'when' %s: %s", baWhen[i].Tree.Root.String(), err)
					}
					if !cond {
						level.Debug(logger).Log(
							"collid", CollectorId(symtab, logger),
							"script", ScriptName(symtab, logger),
							"name", ba.GetName(symtab, logger),
							"msg", fmt.Sprintf("skipped: when condition false: '%s'", baWhen[i].Tree.Root.String()))

						valid_value = false
						break
					}
				}
				if !valid_value {
					continue
				}
			}
			err := customAction(script, symtab, logger)
			if err != nil {
				return err
			}
		}
	} else {
		idx := 0
		for {
			if set_loops_var {
				preserve_sym_tab(symtab, old_values, "loop_var_idx", idx)
				// symtab["loop_var_idx"] = idx
			}
			valid_value := true

			baUntil := ba.GetUntil()
			for i, condTpl := range baUntil {
				// tmp_res := new(strings.Builder)
				// err := ((*template.Template)(condTpl)).Execute(tmp_res, &symtab)
				cond, err := evalCond(symtab, condTpl)
				if err != nil {
					err := fmt.Errorf("invalid template value for 'until' %s: %s", baUntil[i].Tree.Root.String(), err)
					level.Warn(logger).Log(
						"collid", CollectorId(symtab, logger),
						"script", ScriptName(symtab, logger),
						"name", ba.GetName(symtab, logger),
						"msg", err)
					return err
				}
				if !cond {
					level.Debug(logger).Log(
						"collid", CollectorId(symtab, logger),
						"script", ScriptName(symtab, logger),
						"name", ba.GetName(symtab, logger),
						"msg", fmt.Sprintf("Name: '%s' until limit cond reached : '%s'", ba.GetName(symtab, logger), baUntil[i].Tree.Root.String()))

					valid_value = false
					break
				}
			}
			if !valid_value || idx >= script.UntilLimit {
				if idx >= script.UntilLimit {
					level.Warn(logger).Log(
						"collid", CollectorId(symtab, logger),
						"script", ScriptName(symtab, logger),
						"name", ba.GetName(symtab, logger),
						"msg", fmt.Sprintf("max iteration reached for until action (%d)", script.UntilLimit))
				}
				break
			}
			idx += 1

			err := customAction(script, symtab, logger)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// ***************************************************************************************
// ***************************************************************************************
// yaml script parser
// ***************************************************************************************
// ***************************************************************************************

// parse all possible actions in a yaml script.
//
// * debug: msg: test{{ go template}}
//
// * set_fact: vars to set in symbols table
//
// * query: play url to the target and set resulting json obj the sybols table
//
// * metrics: define a list of metric (metric_name) to generate for the exporter
//
// * actions: define a list of subaction to play: query, metrics...
//
// * play_script: play the script
//
// * metric_name: a metric definition
type tmpActions []map[string]yaml.Node

func build_WithItems(raw yaml.Node) ([]any, error) {
	var listElmt []any
	if raw.Tag == "!!str" {
		with_field, err := NewField(raw.Value, nil)
		if err != nil {
			return nil, fmt.Errorf("invalid template for var name (set_fact) %q: %s", raw.Value, err)
		}
		listElmt = make([]any, 1)
		listElmt[0] = with_field
	} else if raw.Tag == "!!map" {
		var (
			raw_mapElmt map[string]any
			mapElmt     map[any]any
			err         error
		)

		if err = raw.Decode(&raw_mapElmt); err != nil {
			return nil, err
		}
		if mapElmt, err = buildMapField(raw_mapElmt); err != nil {
			return nil, err
		}
		listElmt = make([]any, 1)
		listElmt[0] = mapElmt
	} else if raw.Tag == "!!seq" {
		var (
			str_listElmt []any
			err          error
		)

		if err = raw.Decode(&str_listElmt); err != nil {
			return nil, err
		}
		if listElmt, err = buildSliceField(str_listElmt); err != nil {
			return nil, err
		}
	} else {
		listElmt = make([]any, 0)
	}
	return listElmt, nil
}

func build_Cond(script *YAMLScript, raw yaml.Node) ([]*exporterTemplate, error) {
	var listElmt []string
	var when []*exporterTemplate

	if raw.Tag == "!!str" {
		listElmt = make([]string, 1)
		listElmt[0] = raw.Value
	} else if raw.Tag == "!!seq" {
		if err := raw.Decode(&listElmt); err != nil {
			return nil, err
		}
	} else {
		listElmt = make([]string, 0)
	}
	if len(listElmt) > 0 {
		when = make([]*exporterTemplate, len(listElmt))
		for i, cond := range listElmt {
			var tmpl *template.Template
			var err error

			if !strings.Contains(cond, "{{") {
				cond = "{{ " + cond + " }}"
			}
			// alter the when conditions to add custom templates/funcs if exist
			if script.customTemplate != nil {
				tmpl, err = script.customTemplate.Clone()
				if err != nil {
					return nil, fmt.Errorf("for script %s template clone error: %s", script.name, err)
				}
			} else {
				tmpl = template.New("cond").Funcs(mymap())
			}
			tmpl, err = tmpl.Parse(cond)
			if err != nil {
				return nil, fmt.Errorf("for script %s template %s is invalid: %s", script.name, cond, err)
			}
			when[i] = (*exporterTemplate)(tmpl)
		}
	}
	return when, nil
}

func buildMapField(raw_maps map[string]any) (map[any]any, error) {
	var err error
	final_res := make(map[any]any)
	for key, r_value := range raw_maps {
		res, err := buildFields(key, r_value)
		if err != nil {
			return nil, fmt.Errorf("invalid template for var key %s: %s", key, err)
		}
		for key, val := range res {
			final_res[key] = val
		}
	}
	return final_res, err
}

func buildSliceField(raw_slice []any) ([]any, error) {
	var err error
	final_res := make([]any, len(raw_slice))
	for idx, r_value := range raw_slice {
		res, err := buildValueField(r_value)
		if err != nil {
			return nil, fmt.Errorf("invalid template for var key %q: %s", res, err)
		}
		final_res[idx] = res

	}
	return final_res, err
}

func buildFields(key string, val any) (map[any]any, error) {
	var err error
	var key_field *Field
	var value_field any
	// Check required fields
	res := make(map[any]any)

	key_field, err = NewField(key, nil)
	if err != nil {
		return nil, fmt.Errorf("invalid template for var name (set_fact) %q: %s", key, err)
	}
	value_field, err = buildValueField(val)
	if err != nil {
		return nil, fmt.Errorf("invalid template for var name (set_fact) %q: %s", key, err)
	}
	res[key_field] = value_field

	return res, nil
}

func buildValueField(val any) (any, error) {

	switch curval := val.(type) {
	case string:
		value_field, err := NewField(curval, nil)
		if err != nil {
			return nil, fmt.Errorf("invalid template for var value (set_fact) %q: %s", curval, err)
		}
		return value_field, nil

	// value is a map
	case map[string]any:
		tmp, err := buildMapField(curval)
		if err != nil {
			return nil, fmt.Errorf("invalid template for map value (set_fact) %q: %s", curval, err)
		}
		return tmp, nil
	// value is a slice
	case []any:
		tmp, err := buildSliceField(curval)
		if err != nil {
			return nil, fmt.Errorf("invalid template for map value (set_fact) %q: %s", curval, err)
		}
		return tmp, nil

	// a value int float... what else ?
	default:
		return curval, nil
	}
}

// UnmarshalYAML implements the yaml.Unmarshaler interface for YAMLScript.
func (script *YAMLScript) UnmarshalYAML(value *yaml.Node) error {
	var tmp tmpActions
	if err := value.Decode(&tmp); err != nil {
		return err
	}
	actions, err := ActionsListDecode(script, make(ActionsList, 0, len(tmp)), tmp, value)
	if err != nil {
		return err
	}
	script.Actions = actions
	script.UntilLimit = 20
	return nil
}

func ActionsListDecode(script *YAMLScript, actions ActionsList, tmp tmpActions, parentNode *yaml.Node) (ActionsList, error) {
	main_checker := map[string]bool{
		"name":       true,
		"loop":       true,
		"loop_var":   true,
		"vars":       true,
		"until":      true,
		"when":       true,
		"with_items": true,
	}

	for i := range tmp {
		var name *Field
		var nameVal, loopVar string
		var with_items []any
		var until, when []*exporterTemplate
		var vars [][]any
		var err error
		checker := main_checker
		skip_checker := false
		cur_act := tmp[i]

		// parse name
		if raw, ok := cur_act["name"]; ok {
			nameVal = raw.Value
			name, err = NewField(nameVal, nil)
			if err != nil {
				return nil, fmt.Errorf("name for action invalid %q: %s", raw.Value, err)
			}
		}
		// parse vars
		if raw, ok := cur_act["vars"]; ok {
			if raw.Tag == "!!map" {
				var tmp_vars map[string]any
				if err := raw.Decode(&tmp_vars); err != nil {
					return nil, err
				}
				idx := 0
				vars = make([][]any, len(tmp_vars))
				for key, val := range tmp_vars {
					// Check required fields
					vars[idx] = make([]any, 2)
					if new_map, err := buildFields(key, val); err != nil {
						return nil, err
					} else {
						for key, val := range new_map {
							vars[idx][0] = key
							vars[idx][1] = val
						}
					}
					idx++
				}
			}
		}

		// parse with_items
		if raw, ok := cur_act["with_items"]; ok {
			listElmt, err := build_WithItems(raw)
			if err != nil {
				return nil, err
			}
			with_items = listElmt
		} else if raw, ok := cur_act["loop"]; ok {
			listElmt, err := build_WithItems(raw)
			if err != nil {
				return nil, err
			}
			with_items = listElmt
		}
		if with_items != nil {
			// parse loop_var
			if raw, ok := cur_act["loop_var"]; ok {
				loopVar = raw.Value
			}
		}

		// parse when
		if raw, ok := cur_act["when"]; ok {
			cond, err := build_Cond(script, raw)
			if err != nil {
				return nil, err
			}
			when = cond
		}

		// parse when
		if raw, ok := cur_act["until"]; ok {
			cond, err := build_Cond(script, raw)
			if err != nil {
				return nil, err
			}
			until = cond
		}

		// ***********************************************
		// ***********************************************
		// ** parse the action keyword
		// ***********************************************
		// ***********************************************

		// ***********************************************
		// debug
		if raw, ok := cur_act["debug"]; ok {
			checker["debug"] = true
			da := &DebugActionConfig{}
			if err := raw.Decode(da); err != nil {
				err = fmt.Errorf("%v: for action '%s'", err, name.String())
				return nil, err
			}
			a := &DebugAction{}
			a.Debug = da
			if err = a.setBasicElement(name, vars, with_items, loopVar, when, until); err != nil {
				return nil, err
			}
			actions = append(actions, a)
		} else if raw, ok := cur_act["set_fact"]; ok {
			// ***********************************************
			// set_fact
			checker["set_fact"] = true
			sfa := make(map[string]interface{})
			if err := raw.Decode(&sfa); err != nil {
				err = fmt.Errorf("%v: for action '%s'", err, name.String())
				return nil, err
			}
			a := &SetFactAction{}
			if len(sfa) > 0 {
				a.SetFact = sfa
				a.setFact = make([][]any, len(a.SetFact))

				idx := 0
				for key, val := range a.SetFact {
					// Check required fields
					a.setFact[idx] = make([]any, 2)
					if new_map, err := buildFields(key, val); err != nil {
						return nil, err
					} else {
						for key, val := range new_map {
							a.setFact[idx][0] = key
							a.setFact[idx][1] = val
						}
					}
					idx++
				}
			}
			if err = a.setBasicElement(name, vars, with_items, loopVar, when, until); err != nil {
				return nil, err
			}
			actions = append(actions, a)
		} else if raw, ok := cur_act["query"]; ok {
			// ***********************************************
			// url/query
			checker["query"] = true
			qa := &QueryActionConfig{}
			if err := raw.Decode(qa); err != nil {
				err = fmt.Errorf("%v: for action '%s'", err, name.String())
				return nil, err
			}
			a := &QueryAction{}
			a.Query = qa
			if err = a.setBasicElement(name, vars, with_items, loopVar, when, until); err != nil {
				return nil, err
			}
			actions = append(actions, a)
		} else if raw, ok := cur_act["play_script"]; ok {
			// ***********************************************
			// play_script
			checker["play_script"] = true
			script_name := ""
			if err := raw.Decode(&script_name); err != nil {
				err = fmt.Errorf("%v: for action '%s'", err, name.String())
				return nil, err
			}
			a := &PlayScriptAction{
				PlayScriptActionName: script_name,
			}
			if err = a.setBasicElement(name, vars, with_items, loopVar, when, until); err != nil {
				return nil, err
			}
			actions = append(actions, a)
		} else if _, ok := cur_act["metric_name"]; ok {
			// ***********************************************
			// play_script
			checker["metric_name"] = true
			mc := &MetricConfig{}
			if err := parentNode.Content[i].Decode(mc); err != nil {
				return nil, err
			}
			// MAYBE mc.Name should be a Field so that the name could be a template !!
			a := &MetricAction{
				mc: mc,
			}
			name, err = NewField(mc.Name, nil)
			if err != nil {
				return nil, fmt.Errorf("name for action invalid %q: %s", mc.Name, err)
			}

			if err = a.setBasicElement(name, vars, with_items, loopVar, when, until); err != nil {
				return nil, err
			}
			actions = append(actions, a)
			skip_checker = true
		} else if raw, ok := cur_act["actions"]; ok {
			// ***********************************************
			// actions
			checker["actions"] = true
			if raw.Tag == "!!seq" {
				var tmp_sub tmpActions
				if err := raw.Decode(&tmp_sub); err != nil {
					err = fmt.Errorf("%v: for action '%s'", err, name.String())
					return nil, err
				}

				acta, err := ActionsListDecode(script, make(ActionsList, 0, len(tmp_sub)), tmp_sub, &raw)
				if err != nil {
					return nil, err
				}
				a := &ActionsAction{}
				a.Actions = acta
				if err = a.setBasicElement(name, vars, with_items, loopVar, when, until); err != nil {
					return nil, err
				}
				actions = append(actions, a)
			}
		} else if raw, ok := cur_act["metrics"]; ok {
			// ***********************************************
			// metric name
			checker["metrics"] = true
			checker["scope"] = true
			checker["metric_prefix"] = true

			if raw.Tag == "!!seq" {
				var tmp_sub tmpActions
				if err := raw.Decode(&tmp_sub); err != nil {
					err = fmt.Errorf("%v: for action '%s'", err, name.String())
					return nil, err
				}

				acta, err := ActionsListDecode(script, make(ActionsList, 0, len(tmp_sub)), tmp_sub, &raw)
				if err != nil {
					return nil, err
				}
				a := &MetricsAction{}
				a.Actions = acta
				if err = a.setBasicElement(name, vars, with_items, loopVar, when, until); err != nil {
					return nil, err
				}
				actions = append(actions, a)

				//*** append current metrics list to the global list
				script.metricsActions = append(script.metricsActions, a)

				mcl := make([]*MetricConfig, len(raw.Content))
				idx := 0
				for _, act := range acta {
					if act.Type() == metric_action {
						mcl[idx] = act.GetMetric()
						idx++
					}
				}
				a.Metrics = mcl

				if raw, ok := cur_act["scope"]; ok {
					if raw.Tag == "!!str" {
						scope := ""
						if err := raw.Decode(&scope); err != nil {
							return nil, err
						}
						a.Scope = scope
					}
					// # propagate scope == "none" to all metrics
					if a.Scope == "none" {
						for _, mcl := range a.Metrics {
							if mcl.Scope == "" {
								mcl.Scope = a.Scope
							}
						}
					}
				}

				if raw, ok := cur_act["metric_prefix"]; ok {
					if raw.Tag == "!!str" {
						metric_prefix := ""
						if err := raw.Decode(&metric_prefix); err != nil {
							return nil, err
						}
						a.MetricPrefix = metric_prefix
					}
				}
			}
		} else if raw, ok := cur_act["set_stats"]; ok {
			// ***********************************************
			// set_fact
			checker["set_stats"] = true
			ssa := make(map[string]interface{})
			if err := raw.Decode(&ssa); err != nil {
				err = fmt.Errorf("%v: for action '%s'", err, name.String())
				return nil, err
			}
			a := &SetStatsAction{}
			if len(ssa) > 0 {
				a.SetStats = ssa
				a.setStats = make([][]any, len(a.SetStats))

				idx := 0
				for key, val := range a.SetStats {
					// Check required fields
					a.setStats[idx] = make([]any, 2)
					if new_map, err := buildFields(key, val); err != nil {
						return nil, err
					} else {
						for key, val := range new_map {
							a.setStats[idx][0] = key
							a.setStats[idx][1] = val
						}
					}
					idx++
				}
			}
			if err = a.setBasicElement(name, vars, with_items, loopVar, when, until); err != nil {
				return nil, err
			}
			actions = append(actions, a)

			//*** append current metrics list to the global list
			script.setStatsActions = append(script.setStatsActions, a)
		} else {
			// we haven't found any label in action that we should understand
			// display first key of the map and context (line, column)
			for key, val := range cur_act {
				err := fmt.Errorf("unknown action type: '%s': '%v', arround line %d column: %d", key, val.Value, val.Line, val.Column)
				return nil, err
			}
			// ***********************************************
			// return nil, fmt.Errorf("unknown action type: +%v", cur_act)
		}

		if !skip_checker {
			for name, raw := range cur_act {
				if _, ok := checker[name]; !ok {
					return nil, fmt.Errorf("unknown attribute '%s' for action '%v' on line: %d column: %d", name, reflect.TypeOf(actions[len(actions)-1]), raw.Line, raw.Column)

				}
			}
		}
	}
	return actions, nil
}

// ***************************************************************************************
