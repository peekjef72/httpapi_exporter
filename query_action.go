package main

import (
	//"bytes"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
)

// ***************************************************************************************
// ***************************************************************************************
// query
// ***************************************************************************************
// ***************************************************************************************
type QueryActionConfig struct {
	Query      string             `yaml:"url"`
	Method     string             `yaml:"method,omitempty"`
	Data       string             `yaml:"data,omitempty"`
	Debug      ConvertibleBoolean `yaml:"debug,omitempty"`
	VarName    string             `yaml:"var_name,omitempty"`
	OkStatus   any                `yaml:"ok_status,omitempty"`
	AuthConfig *AuthConfig        `yaml:"auth_config,omitempty"`
	Timeout    int                `yaml:"timeout,omitempty"`

	query    *Field
	method   *Field
	data     *Field
	var_name *Field

	auth_mode *Field
	user      *Field
	passwd    *Field
	token     *Field

	ok_status []int

	// Catches all undefined fields and must be empty after parsing.
	XXX map[string]interface{} `yaml:",inline" json:"-"`
}

// UnmarshalYAML implements the yaml.Unmarshaler interface for QueryActionConfig.
func (qc *QueryActionConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type plain QueryActionConfig
	var err error
	if err := unmarshal((*plain)(qc)); err != nil {
		return err
	}
	// Check required fields
	qc.query, err = NewField(qc.Query, nil)
	if err != nil {
		return fmt.Errorf("invalid template for query %q: %s", qc.Query, err)
	}

	if qc.Method == "" {
		qc.Method = "GET"
	}
	qc.method, err = NewField(qc.Method, nil)
	if err != nil {
		return fmt.Errorf("invalid template for method %q: %s", qc.Method, err)
	}
	if qc.Data != "" {
		qc.data, err = NewField(qc.Data, nil)
		if err != nil {
			return fmt.Errorf("invalid template for data %q: %s", qc.Data, err)
		}
	} else {
		qc.data = nil
	}

	if qc.VarName == "" {
		qc.VarName = "_root"
	}
	qc.var_name, err = NewField(qc.VarName, nil)
	if err != nil {
		return fmt.Errorf("invalid template for var_name %q: %s", qc.VarName, err)
	}

	if qc.OkStatus != nil {
		qc.ok_status = buildStatus(qc.OkStatus)
	}
	// set default if something was wrong or not set
	if qc.ok_status == nil {
		qc.ok_status = []int{200}
	}

	if qc.AuthConfig != (*AuthConfig)(nil) {
		if qc.AuthConfig.Mode != "" {
			qc.auth_mode, err = NewField(qc.AuthConfig.Mode, nil)
			if err != nil {
				return fmt.Errorf("invalid template for query auth_mode %q: %s", qc.AuthConfig.Mode, err)
			}
		}
		if qc.AuthConfig.Username != "" {
			qc.user, err = NewField(qc.AuthConfig.Username, nil)
			if err != nil {
				return fmt.Errorf("invalid template for query username %q: %s", qc.AuthConfig.Username, err)
			}
		}
		if qc.AuthConfig.Password != "" {
			qc.passwd, err = NewField(string(qc.AuthConfig.Password), nil)
			if err != nil {
				return fmt.Errorf("invalid template for query password %q: %s", qc.AuthConfig.Password, err)
			}
		}
		if qc.AuthConfig.Token != "" {
			qc.token, err = NewField(string(qc.AuthConfig.Token), nil)
			if err != nil {
				return fmt.Errorf("invalid template for query auth_token %q: %s", qc.AuthConfig.Token, err)
			}
		}
	}

	return checkOverflow(qc.XXX, "query action")
}

func buildStatus(raw_status any) []int {
	var status []int
	switch curval := raw_status.(type) {
	case int:
		status = make([]int, 1)
		status[0] = curval
	case []any:
		status = make([]int, len(curval))
		for idx, subval := range curval {
			switch sub_val := subval.(type) {
			case int:
				status[idx] = sub_val
			case string:
				var i_value int64
				var err error
				if i_value, err = strconv.ParseInt(strings.Trim(sub_val, "\r\n "), 10, 0); err != nil {
					i_value = 0
				}
				status[idx] = int(i_value)
			}
		}
	}
	return status
}

type QueryAction struct {
	Name    *Field              `yaml:"name,omitempty"`
	With    []any               `yaml:"with,omitempty"`
	When    []*exporterTemplate `yaml:"when,omitempty"`
	LoopVar string              `yaml:"loop_var,omitempty"`
	Vars    [][]any             `yaml:"vars,omitempty"`
	Until   []*exporterTemplate `yaml:"until,omitempty"`
	Query   *QueryActionConfig

	vars [][]any

	// Catches all undefined fields and must be empty after parsing.
	XXX map[string]interface{} `yaml:",inline" json:"-"`
}

// *****************
func (a *QueryAction) Type() int {
	return query_action
}

func (a *QueryAction) GetName(symtab map[string]any, logger log.Logger) string {
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
func (a *QueryAction) GetNameField() *Field {
	return a.Name
}
func (a *QueryAction) SetNameField(name *Field) {
	a.Name = name
}

func (a *QueryAction) GetWidh() []any {
	return a.With
}
func (a *QueryAction) SetWidth(with []any) {
	a.With = with
}

func (a *QueryAction) GetWhen() []*exporterTemplate {
	return a.When

}
func (a *QueryAction) SetWhen(when []*exporterTemplate) {
	a.When = when
}

func (a *QueryAction) GetLoopVar() string {
	return a.LoopVar
}
func (a *QueryAction) SetLoopVar(loopvar string) {
	a.LoopVar = loopvar
}

func (a *QueryAction) GetVars() [][]any {
	return a.vars
}
func (a *QueryAction) SetVars(vars [][]any) {
	a.vars = vars
}

func (a *QueryAction) GetUntil() []*exporterTemplate {
	return a.Until
}
func (a *QueryAction) SetUntil(until []*exporterTemplate) {
	a.Until = until
}

func (a *QueryAction) setBasicElement(
	nameField *Field,
	vars [][]any,
	with []any,
	loopVar string,
	when []*exporterTemplate,
	until []*exporterTemplate) error {
	return setBasicElement(a, nameField, vars, with, loopVar, when, until)
}

func (a *QueryAction) PlayAction(script *YAMLScript, symtab map[string]any, logger log.Logger) error {
	return PlayBaseAction(script, symtab, logger, a, a.CustomAction)
}

// only for MetricsAction
func (a *QueryAction) GetMetrics() []*GetMetricsRes {
	return nil
}

// only for MetricAction
func (a *QueryAction) GetMetric() *MetricConfig {
	return nil
}
func (a *QueryAction) SetMetricFamily(*MetricFamily) {
}

// only for PlayAction
func (a *QueryAction) SetPlayAction(scripts map[string]*YAMLScript) error {
	return nil
}

func (a *QueryAction) AddCustomTemplate(customTemplate *exporterTemplate) error {

	if err := AddCustomTemplate(a, customTemplate); err != nil {
		return err
	}

	if a.Query.query != nil {
		if err := a.Query.query.AddDefaultTemplate(customTemplate); err != nil {
			return err
		}
	}

	if a.Query.method != nil {
		if err := a.Query.method.AddDefaultTemplate(customTemplate); err != nil {
			return err
		}
	}

	if a.Query.data != nil {
		if err := a.Query.data.AddDefaultTemplate(customTemplate); err != nil {
			return err
		}
	}
	if a.Query.var_name != nil {
		if err := a.Query.var_name.AddDefaultTemplate(customTemplate); err != nil {
			return err
		}
	}

	if a.Query.auth_mode != nil {
		if err := a.Query.auth_mode.AddDefaultTemplate(customTemplate); err != nil {
			return err
		}
	}

	if a.Query.user != nil {
		if err := a.Query.user.AddDefaultTemplate(customTemplate); err != nil {
			return err
		}
	}

	if a.Query.passwd != nil {
		if err := a.Query.passwd.AddDefaultTemplate(customTemplate); err != nil {
			return err
		}
	}

	if a.Query.token != nil {
		if err := a.Query.token.AddDefaultTemplate(customTemplate); err != nil {
			return err
		}
	}

	return nil
}

// specific behavior for the QueryAction
func (a *QueryAction) CustomAction(script *YAMLScript, symtab map[string]any, logger log.Logger) error {
	var (
		err                                 error
		payload, query, method, var_name    string
		auth_mode, user, passwd, auth_token string
	)

	level.Debug(logger).Log(
		"collid", CollectorId(symtab, logger),
		"script", ScriptName(symtab, logger),
		"name", a.GetName(symtab, logger),
		"msg", "[Type: QueryAction]")

	query, err = a.Query.query.GetValueString(symtab, nil, false)
	if err != nil {
		query = a.Query.Query
		level.Warn(logger).Log(
			"collid", CollectorId(symtab, logger),
			"script", ScriptName(symtab, logger),
			"name", a.GetName(symtab, logger),
			"msg", fmt.Sprintf("invalid template for query '%s': %v", a.Query.Query, err))
	}

	if a.Query.data != nil {
		payload, err = a.Query.data.GetValueString(symtab, nil, false)
		if err != nil {
			payload = a.Query.Data
			level.Warn(logger).Log(
				"collid", CollectorId(symtab, logger),
				"script", ScriptName(symtab, logger),
				"name", a.GetName(symtab, logger),
				"msg", fmt.Sprintf("invalid template for data '%s': %v", a.Query.Data, err))
		}
	} else {
		payload = ""
	}

	method, err = a.Query.method.GetValueString(symtab, nil, false)
	if err != nil {
		method = strings.ToUpper(a.Query.Method)
		level.Warn(logger).Log(
			"collid", CollectorId(symtab, logger),
			"script", ScriptName(symtab, logger),
			"name", a.GetName(symtab, logger),
			"msg", fmt.Sprintf("invalid template for method '%s': %v", a.Query.Method, err))
	}

	var_name, err = a.Query.var_name.GetValueString(symtab, nil, false)
	if err != nil {
		level.Warn(logger).Log(
			"collid", CollectorId(symtab, logger),
			"script", ScriptName(symtab, logger),
			"name", a.GetName(symtab, logger),
			"msg", fmt.Sprintf("invalid template for var_name '%s': %v", a.Query.VarName, err))
	}

	auth_mode, err = a.Query.auth_mode.GetValueString(symtab, nil, false)
	if err != nil {
		level.Warn(logger).Log(
			"collid", CollectorId(symtab, logger),
			"script", ScriptName(symtab, logger),
			"name", a.GetName(symtab, logger),
			"msg", fmt.Sprintf("invalid template for auth_config.mode '%s': %v", a.Query.AuthConfig.Mode, err))
	}

	user, err = a.Query.user.GetValueString(symtab, nil, false)
	if err != nil {
		level.Warn(logger).Log(
			"collid", CollectorId(symtab, logger),
			"script", ScriptName(symtab, logger),
			"name", a.GetName(symtab, logger),
			"msg", fmt.Sprintf("invalid template for auth_config.user '%s': %v", a.Query.AuthConfig.Username, err))
	}

	passwd, err = a.Query.passwd.GetValueString(symtab, nil, false)
	if err != nil {
		level.Warn(logger).Log(
			"collid", CollectorId(symtab, logger),
			"script", ScriptName(symtab, logger),
			"name", a.GetName(symtab, logger),
			"msg", fmt.Sprintf("invalid template for auth_config.user '%s': %v", a.Query.AuthConfig.Password, err))
	}

	auth_token, err = a.Query.token.GetValueString(symtab, nil, false)
	if err != nil {
		level.Warn(logger).Log(
			"collid", CollectorId(symtab, logger),
			"script", ScriptName(symtab, logger),
			"name", a.GetName(symtab, logger),
			"msg", fmt.Sprintf("invalid template for auth_config.token '%s': %v", a.Query.AuthConfig.Token, err))
	}

	params := &CallClientExecuteParams{
		Payload:  payload,
		Method:   method,
		Url:      query,
		Debug:    bool(a.Query.Debug),
		VarName:  var_name,
		OkStatus: a.Query.ok_status,
		AuthMode: auth_mode,
		Username: user,
		Password: passwd,
		Token:    auth_token,
		Timeout:  time.Duration(a.Query.Timeout) * time.Second,
	}

	level.Debug(logger).Log(
		"collid", CollectorId(symtab, logger),
		"script", ScriptName(symtab, logger),
		"name", a.GetName(symtab, logger),
		"msg", fmt.Sprintf("    query: '%s' - method: '%s' - target_var: '%s'", query, method, a.Query.VarName))

	if raw_func, ok := symtab["__method"]; ok {
		if Func, ok := raw_func.(func(*CallClientExecuteParams, map[string]any) error); ok {
			if err = Func(params, symtab); err != nil {
				if err != ErrInvalidLogin || err == ErrInvalidLoginNoCipher || err == ErrInvalidLoginInvalidCipher {
					switch err {
					case ErrContextDeadLineExceeded:
						level.Warn(logger).Log(
							"collid", CollectorId(symtab, logger),
							"script", ScriptName(symtab, logger),
							"name", a.GetName(symtab, logger),
							"msg", fmt.Sprintf("internal method returns error: '%v'", err),
							"timeout", fmt.Sprintf("%v", a.Query.Timeout))
					default:
						level.Warn(logger).Log(
							"collid", CollectorId(symtab, logger),
							"script", ScriptName(symtab, logger),
							"name", a.GetName(symtab, logger),
							"msg", fmt.Sprintf("internal method returns error: '%v'", err))
					}
				}
			}
		}
	} else {
		level.Warn(logger).Log(
			"collid", CollectorId(symtab, logger),
			"script", ScriptName(symtab, logger),
			"msg", "internal method to play not found")
	}
	return err
}

// ***************************************************************************************
