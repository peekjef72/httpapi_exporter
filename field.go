// cSpell:ignore tmpl, mytemplate, jscode, vartype, curval, lenattr, mapkey
package main

import (
	"encoding/json"
	"fmt"
	"html"
	"log/slog"
	"reflect"
	"strconv"

	"github.com/peekjef72/httpapi_exporter/goja_modules"

	"github.com/spf13/cast"

	ttemplate "text/template"

	mytemplate "github.com/peekjef72/httpapi_exporter/template"

	"strings"
)

type exporterTemplate ttemplate.Template

func (tmpl *exporterTemplate) MarshalText() (text []byte, err error) {

	return []byte(tmpl.Tree.Root.String()), nil
}

type Field struct {
	raw     string
	vartype int
	tmpl    *exporterTemplate
	jscode  *goja_modules.JSCode
}

const (
	field_raw = iota
	field_var
	field_template
	field_js
)

// create a new key or value Field that can be a GO template
func NewField(name string, customTemplate *exporterTemplate) (*Field, error) {
	var (
		tmpl    *ttemplate.Template
		err     error
		vartype int = field_raw
		jscode  *goja_modules.JSCode
	)

	name = strings.TrimSpace(name)
	if strings.Contains(name, "{{") {
		if customTemplate != nil {
			ptr := (*ttemplate.Template)(customTemplate)
			tmpl, err = ptr.Clone()
			if err != nil {
				return nil, fmt.Errorf("field template clone for %s is invalid: %s", name, err)
			}
		} else {
			tmpl = ttemplate.New("field").Funcs(mytemplate.MyMap())
		}
		tmpl, err = tmpl.Parse(name)
		if err != nil {
			return nil, fmt.Errorf("field template %s is invalid: %s", name, err)
		}
		if tmpl != nil {
			vartype = field_template
		}
	} else if strings.HasPrefix(name, "$") {
		if name[0] == '$' {
			name = name[1:]
		}
		vartype = field_var
	} else if strings.HasPrefix(name, "js:") {
		code := strings.TrimPrefix(name, "js:")

		jscode, err = goja_modules.NewJSCode(code, mytemplate.Js_func_map())
		if err != nil {
			return nil, fmt.Errorf("field js code %s is invalid: %s", code, err)
		}
		if jscode != nil {
			vartype = field_js
		}
	}
	return &Field{
		raw:     name,
		vartype: vartype,
		tmpl:    (*exporterTemplate)(tmpl),
		jscode:  jscode,
	}, nil
}

// obtain float64 from a var of any type
func RawGetValueFloat(curval any) float64 {
	var f_value float64 = 0.0

	if curval != nil {
		f_value = cast.ToFloat64(curval)
	}
	return f_value
}

// obtain string from a var of any type
func RawGetValueString(curval any) string {
	res := ""
	if curval != nil {
		vSrc := reflect.ValueOf(curval)
		switch vSrc.Kind() {

		case reflect.Map:
			fallthrough
		case reflect.Slice:
			res_b, err := json.Marshal(curval)
			if err != nil {
				return ""
			}
			res = string(res_b)

		default:
			res = cast.ToString(curval)
		}
	}

	return strings.Trim(res, "\r\n ")
}

func ConvertToBool(value_raw any) (value bool) {
	switch value_val := value_raw.(type) {
	case string:
		asString := strings.ToLower(value_val)
		switch asString {
		case "1", "t", "true", "on", "yes":
			value = true
		}
	default:
		value = cast.ToBool(value_raw)
	}
	return
}

func (f *Field) GetValueSimple(
	item map[string]interface{},
	logger *slog.Logger,
	out_type reflect.Kind,
) (res any, err error) {

	var raw_data any

	if f == nil {
		return "", nil
	}

	switch f.vartype {
	case field_template:
		defer func() {
			// res and err are named out parameters, so if we set value for them in defer
			// set the returned values
			ok := false
			if r := recover(); r != nil {
				if err, ok = r.(error); !ok {
					err = newVarError(error_var_invalid_template,
						fmt.Sprintf("panic in GetValueString template with undefined error: %s", err.Error()))
				}
				res = ""
			}
		}()
		tmp_res := new(strings.Builder)
		err = ((*ttemplate.Template)(f.tmpl)).Execute(tmp_res, &item)
		if err != nil {
			return "", newVarError(error_var_invalid_template,
				fmt.Sprintf("invalid template: %s", err.Error()))
		}
		// obtain final string from builder
		tmp := tmp_res.String()
		// remove before and after blank chars
		tmp = strings.Trim(tmp, "\r\n ")
		// unescape string
		raw_data = html.UnescapeString(tmp)
	case field_var:
		data, err := getVar(item, f.raw, logger)
		if err != nil {
			return "", err
		}
		raw_data = data

	case field_js:
		val, err := f.jscode.Run(item, logger)
		if err != nil {
			return "", newVarError(error_var_invalid_javascript_code,
				fmt.Sprintf("invalid javascript code execution: %s", err.Error()))
		}
		raw_data = val
	default:
		raw_data = f.raw
	}
	switch out_type {
	case reflect.String:
		res = RawGetValueString(raw_data)
	case reflect.Float64:
		res = RawGetValueFloat(raw_data)
	case reflect.Int64:
		var i_value int64 = 0

		if raw_data != nil {
			i_value = cast.ToInt64(raw_data)
		}
		res = i_value
	case reflect.Uint64:
		var u_value uint64 = 0

		if raw_data != nil {
			u_value = cast.ToUint64(raw_data)
		}
		res = u_value
	case reflect.Bool:
		if raw_data != nil {
			res = ConvertToBool(raw_data)
		}
	}
	return res, nil
}

// obtain a final string value from Field
// use template if one is defined using item to symbols table
// else
// check if value must be substituted using provided sub map
// if check_item set to true, check if the resulting value exists in item symbols table
// else return raw value (simple string)
func (f *Field) GetValueString(
	symtab map[string]interface{},
	logger *slog.Logger,
) (res string, err error) {
	raw_str, err := f.GetValueSimple(symtab, logger, reflect.String)
	if err != nil {
		res = ""
		return
	}
	if str, ok := raw_str.(string); ok {
		res = str
	}
	return
}

// obtain a final float64 value from Field
// use template if one is defined using item to symbols table
// else if the resulting value exists in item symbols table return it
// else return raw value (simple float64 constant)
func (f *Field) GetValueFloat(
	symtab map[string]interface{},
	logger *slog.Logger,
) (res float64, err error) {

	raw_f, err := f.GetValueSimple(symtab, logger, reflect.Float64)
	if err != nil {
		res = 0.0
		return
	}
	if num, ok := raw_f.(float64); ok {
		res = num
	}
	return
}

// eval field as a boolean condition: return true|false or error if something bad!
func (cond *Field) EvalCond(
	symtab map[string]any,
	logger *slog.Logger,
) (res_cond bool, err error) {

	res_cond = false
	err = nil

	if cond == nil {
		return
	}

	switch cond.vartype {
	case field_template:
		var str_val string
		if str_val, err = cond.GetValueString(symtab, logger); err != nil {
			return
		} else if str_val == "true" {
			res_cond = true
		}
	default:
		var val float64
		if val, err = cond.GetValueFloat(symtab, logger); err != nil {
			return
		} else if val != 0 {
			res_cond = true
		}
	}
	return
}

func getMapKey(raw_value any, key string) any {

	var new_value any

	vSrc := reflect.ValueOf(raw_value)

	if vSrc.Kind() == reflect.Map {
		tmp_value := vSrc.MapIndex(reflect.ValueOf(key))
		if tmp_value.IsValid() {
			new_value = tmp_value.Interface()
		}
	}

	return new_value
}

func getSliceIndex(raw_value any, index int) any {

	var res_value any = nil

	if index != -1 {
		vSrc := reflect.ValueOf(raw_value)
		if vSrc.Kind() == reflect.Slice {
			if index < vSrc.Len() {
				res_value = vSrc.Index(index).Interface()
			}
		}
	}
	return res_value
}

func buildAttrWithIndex(
	symtab map[string]any,
	raw_value any,
	var_name,
	index_str string,
	logger *slog.Logger,
) (any, error) {
	vDst := reflect.ValueOf(raw_value)
	if vDst.Kind() == reflect.Map {
		if index_str[0] == '$' {
			tmp_name, _ := extract_var_name(index_str[1:])
			raw_ind, err := getVar(symtab, tmp_name, logger)
			if err != nil {
				return nil, err
			} else {
				index_str = cast.ToString(raw_ind)
			}
		}
		raw_value = getMapKey(raw_value, index_str)
		if raw_value == nil {
			return nil, newVarError(error_var_mapkey_not_found,
				fmt.Sprintf("key '%s' not found in %s map", index_str, var_name))
		}
	} else if vDst.Kind() == reflect.Slice {
		var index int

		if index_str[0] == '$' {
			tmp_name, _ := extract_var_name(index_str[1:])
			raw_ind, err := getVar(symtab, tmp_name, logger)
			if err != nil {
				return nil, err
			} else {
				index = cast.ToInt(raw_ind)
			}
		} else {
			if i_value, err := strconv.ParseInt(index_str, 10, 0); err != nil {
				index = 0
			} else {
				index = int(i_value)
			}
		}
		raw_value = getSliceIndex(raw_value, index)
		if raw_value == nil {
			return nil, newVarError(error_var_sliceindex_not_found,
				fmt.Sprintf("index '%s' not found in %s array", index_str, var_name))
		}
	}
	return raw_value, nil
}

func extract_var_name(name string) (string, int) {
	var pos int
	list := pat_var_finder.FindStringSubmatch(name)
	if len(list) > 0 {
		name = list[1]
		pos = len(list[0])
	} else {
		pos = len(name)
	}
	return name, pos
}

const (
	error_var_not_found               = iota + 1
	error_var_invalid_type            = iota
	error_var_invalid_template        = iota
	error_var_invalid_json_output     = iota
	error_var_mapkey_not_found        = iota
	error_var_sliceindex_not_found    = iota
	error_var_invalid_javascript_code = iota
)

type varError struct {
	code    int
	message string
}

type VarError interface {
	Code() int
	Error() string
}

func newVarError(code int, msg string) *varError {
	return &varError{
		code:    code,
		message: msg,
	}
}

func (e *varError) Error() string {
	return fmt.Sprintf("getVarError %d: %s", e.code, e.message)
}

func (e *varError) Code() int {
	return e.code
}

// ***************************************************************************************
func getVar(
	symtab map[string]any,
	attr string,
	logger *slog.Logger,
) (any, error) {
	var err error

	tmp_symtab := symtab
	// split the attr string into parts: attr1.attr[0].attr
	if attr[0] == '.' {
		attr = attr[1:]
	}
	vars := strings.Split(attr, ".")
	lenattr := len(vars) - 1
	for idx, var_name := range vars {
		index_str := ""
		// check if attr refers to an another variable name $X.[...].$var_name
		if var_name[0] == '$' {
			tmp_name, pos := extract_var_name(var_name[1:])
			raw_value, err := getVar(symtab, tmp_name, logger)
			if err != nil {
				return nil, err
			}
			if attr_name, ok := raw_value.(string); !ok {
				return nil, newVarError(error_var_invalid_type, fmt.Sprintf("attribute '%s' is not of 'string' type", tmp_name))
			} else {
				// with format ${var_name}[x] with must append [x] to computed attribute var_name => attr_name[x]
				if len(attr_name) < pos {
					var_name = attr_name + var_name[pos+1:]
				} else {
					var_name = attr_name
				}
			}

		}
		// check if component contains index pos: attr[x]
		if pos := strings.Index(var_name, "["); pos != -1 {
			pos2 := strings.Index(var_name, "]")
			index_str = var_name[pos+1 : pos2]
			var_name = var_name[0:pos]
			// remove enclosing string separators if any found
			index_str = strings.Trim(index_str, "'`\"")
		}
		// try to find attribute name as key name of map element
		if raw_value, ok := tmp_symtab[var_name]; ok {
			if value, ok := raw_value.(*Field); ok {
				raw_value, err = value.GetValueObject(symtab, logger)
				if err != nil {
					return nil, err
				}
			}

			// special case attribute name contains '[index_str]'
			if index_str != "" {
				raw_value, err = buildAttrWithIndex(symtab, raw_value, var_name, index_str, logger)
				if err != nil {
					return nil, err
				}
			}

			// attributes chain is not over, so we check if current element is a map so we can go on on attributes
			if idx < lenattr {
				vDst := reflect.ValueOf(raw_value)

				// check it is a map an convert it to map[string]any
				if vDst.Kind() == reflect.Map {
					mAny := make(map[string]any)
					iter := vDst.MapRange()
					for iter.Next() {
						raw_key := iter.Key()
						raw_value := iter.Value()
						mAny[raw_key.String()] = raw_value.Interface()
					}
					tmp_symtab = mAny
				} else {
					tmp_symtab = nil
					err = newVarError(error_var_invalid_type,
						fmt.Sprintf("attribute '%s' is not of 'map' type", var_name))
				}
			} else {
				return raw_value, err
			}
		} else {
			err = newVarError(error_var_not_found,
				fmt.Sprintf("%s not found", var_name))
			tmp_symtab = nil
			break
		}
	}
	return tmp_symtab, err
}

func (f *Field) GetValueObject(
	item any,
	logger *slog.Logger,
) (res any, err error) {
	res_slice := make([]any, 0)

	if f == nil {
		return res_slice, nil
	}

	switch f.vartype {
	case field_template:
		defer func() {
			// res and err are named out parameters, so if we set value for them in defer
			// set the returned values
			ok := false
			if r := recover(); r != nil {
				if err, ok = r.(error); !ok {
					err = newVarError(error_var_invalid_template, fmt.Sprintf("panic in GetValueObject template with undefined error: %s", err.Error()))
				}
				res = res_slice
			}
		}()
		tmp_res := new(strings.Builder)
		err := ((*ttemplate.Template)(f.tmpl)).Execute(tmp_res, &item)
		if err != nil {
			return res_slice, newVarError(error_var_invalid_template, fmt.Sprintf("invalid template: %s", err.Error()))
		}
		var data any
		json_obj := tmp_res.String()
		if json_obj == "<no value>" || json_obj == "" || json_obj == "null" {
			data = ""
		} else {
			json_obj = html.UnescapeString(json_obj)
			err = json.Unmarshal([]byte(json_obj), &data)
			if err != nil {
				if _, ok := err.(*json.SyntaxError); ok {
					// invalid character 'X' in literal true
					return json_obj, nil
				}
				return res_slice, newVarError(error_var_invalid_json_output, fmt.Sprintf("invalid json output format: %s", err.Error()))
			}
		}
		return data, nil
	case field_var:
		if symtab, ok := item.(map[string]any); ok {
			data, err := getVar(symtab, f.raw, logger)
			if err != nil {
				return res_slice, err
			}

			return data, nil
		} else {
			return nil, nil
		}
	case field_js:
		symtab := item.(map[string]any)

		val, err := f.jscode.Run(symtab, logger)
		if err != nil {
			return res_slice, newVarError(error_var_invalid_javascript_code,
				fmt.Sprintf("invalid javascript code execution: %s", err.Error()))
		}
		return val, nil
	}
	// else it is a simple string `value`
	return RawGetValueString(f.raw), nil
}

func (f *Field) String() string {
	if f == nil {
		return ""
	}

	switch f.vartype {
	case field_template:
		return f.tmpl.Tree.Root.String()
	case field_var:
		return "$" + f.raw
	case field_js:
		return f.raw
	}
	return f.raw
}

func (f *Field) MarshalText() (text []byte, err error) {

	return []byte(f.String()), nil
}

func (f *Field) AddDefaultTemplate(customTemplate *exporterTemplate) error {
	if f.vartype == field_template && customTemplate != nil {
		if _, err := AddDefaultTemplate(f, customTemplate); err != nil {
			return err
		}
	}
	return nil
}

func AddDefaultTemplate(dest_cond *Field, customTemplate *exporterTemplate) (*Field, error) {
	if dest_cond != nil && dest_cond.vartype == field_template && customTemplate != nil {
		cc_tmpl, err := ((*ttemplate.Template)(customTemplate)).Clone()
		if err != nil {
			return nil, fmt.Errorf("field template clone for %s is invalid: %s", ((*ttemplate.Template)(customTemplate)).Name(), err)
		}
		for _, tmpl := range cc_tmpl.Templates() {
			name := tmpl.Name()
			if name == "default" {
				continue
			}

			_, err = ((*ttemplate.Template)(dest_cond.tmpl)).AddParseTree(tmpl.Name(), tmpl.Tree)
			if err != nil {
				return nil, fmt.Errorf("field template %s is invalid: %s", tmpl.Name(), err)
			}
		}
	}
	return dest_cond, nil
}
