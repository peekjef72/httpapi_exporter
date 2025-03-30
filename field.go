package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"reflect"
	"strconv"

	"github.com/spf13/cast"

	ttemplate "text/template"

	"strings"
)

type exporterTemplate ttemplate.Template

func (tmpl *exporterTemplate) MarshalText() (text []byte, err error) {

	return []byte(tmpl.Tree.Root.String()), nil
}

type Field struct {
	raw     string
	vartype bool
	tmpl    *exporterTemplate
}

// create a new key or value Field that can be a GO template
func NewField(name string, customTemplate *exporterTemplate) (*Field, error) {
	var (
		tmpl    *ttemplate.Template
		err     error
		vartype bool = false
	)

	if strings.Contains(name, "{{") {
		if customTemplate != nil {
			ptr := (*ttemplate.Template)(customTemplate)
			tmpl, err = ptr.Clone()
			if err != nil {
				return nil, fmt.Errorf("field template clone for %s is invalid: %s", name, err)
			}
		} else {
			tmpl = ttemplate.New("field").Funcs(mymap())
		}
		tmpl, err = tmpl.Parse(name)
		if err != nil {
			return nil, fmt.Errorf("field template %s is invalid: %s", name, err)
		}
	} else if strings.Contains(name, "$") {
		if name[0] == '$' {
			name = name[1:]
		}
		vartype = true
	}
	return &Field{
		raw:     name,
		vartype: vartype,
		tmpl:    (*exporterTemplate)(tmpl),
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
		res = cast.ToString(curval)
	}

	return strings.Trim(res, "\r\n ")
}

// obtain a final string value from Field
// use template if one is defined using item to symbols table
// else
// check if value must be sustituted using provided sub map
// if check_item set to true, check if the resulting value exists in item symbols table
// else return raw value (simple string)
func (f *Field) GetValueString(
	item map[string]interface{},
) (res string, err error) {
	if f == nil {
		return "", nil
	}

	if f.tmpl != nil {
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
		return html.UnescapeString(tmp), nil
	} else {
		if f.vartype {
			data, err := getVar(item, f.raw)
			if err != nil {
				return "", err
			}
			return RawGetValueString(data), nil
		} else {
			val := f.raw
			return RawGetValueString(val), nil
		}
	}
}

// obtain a final float64 value from Field
// use template if one is defined using item to symbols table
// else if the resulting value exists in item symbols table return it
// else return raw value (simple float64 constant)
func (f *Field) GetValueFloat(
	item map[string]interface{}) (res float64, err error) {
	var str_value any

	if f == nil {
		return 0, nil
	}

	if f.tmpl != nil {
		defer func() {
			// res and err are named out parameters, so if we set value for them in defer
			// set the returned values
			ok := false
			if r := recover(); r != nil {
				if err, ok = r.(error); !ok {
					err = errors.New("panic in GetValueFloat template with undefined error")
				}
				res = 0
			}
		}()

		tmp_res := new(strings.Builder)
		err := ((*ttemplate.Template)(f.tmpl)).Execute(tmp_res, &item)
		if err != nil {
			return 0, newVarError(error_var_invalid_template,
				fmt.Sprintf("invalid template: %s", err.Error()))
		}
		str_value = html.UnescapeString(tmp_res.String())
	} else {
		if f.vartype {
			data, err := getVar(item, f.raw)
			if err != nil {
				return 0, err
			}
			str_value = data
		} else {
			val := f.raw
			// check if value exists in symbol table
			if curval, ok := item[val]; ok {
				str_value = curval
			} else {
				str_value = val
			}
		}
	}
	return RawGetValueFloat(str_value), nil
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
	// if raw_value != nil && key != "" {
	// 	switch map_val := raw_value.(type) {
	// 	case map[string]any:
	// 		if tmp_value, ok := map_val[key]; ok {
	// 			new_value = tmp_value
	// 		}
	// 	case map[string]string:
	// 		if tmp_value, ok := map_val[key]; ok {
	// 			new_value = tmp_value
	// 		}
	// 	case map[string]byte:
	// 		if tmp_value, ok := map_val[key]; ok {
	// 			new_value = tmp_value
	// 		}
	// 	case map[string]int:
	// 		if tmp_value, ok := map_val[key]; ok {
	// 			new_value = tmp_value
	// 		}
	// 	case map[string]int64:
	// 		if tmp_value, ok := map_val[key]; ok {
	// 			new_value = tmp_value
	// 		}
	// 	case map[string]float32:
	// 		if tmp_value, ok := map_val[key]; ok {
	// 			new_value = tmp_value
	// 		}
	// 	case map[string]float64:
	// 		if tmp_value, ok := map_val[key]; ok {
	// 			new_value = tmp_value
	// 		}
	// 	}
	// }
	return new_value
}

func getSliceIndice(raw_value any, indice int) any {

	var res_value any = nil

	if indice != -1 {
		vSrc := reflect.ValueOf(raw_value)
		if vSrc.Kind() == reflect.Slice {
			if indice < vSrc.Len() {
				res_value = vSrc.Index(indice).Interface()
			}
		}
		// switch slice_val := raw_value.(type) {
		// case []any:
		// 	if indice < len(slice_val) {
		// 		raw_value = slice_val[indice]
		// 	} else {
		// 		raw_value = nil
		// 	}
		// case []string:
		// 	if indice < len(slice_val) {
		// 		raw_value = slice_val[indice]
		// 	} else {
		// 		raw_value = nil
		// 	}
		// case []byte:
		// 	if indice < len(slice_val) {
		// 		raw_value = slice_val[indice]
		// 	} else {
		// 		raw_value = nil
		// 	}
		// case []int:
		// 	if indice < len(slice_val) {
		// 		raw_value = slice_val[indice]
		// 	} else {
		// 		raw_value = nil
		// 	}
		// case []int64:
		// 	if indice < len(slice_val) {
		// 		raw_value = slice_val[indice]
		// 	} else {
		// 		raw_value = nil
		// 	}
		// case []float32:
		// 	if indice < len(slice_val) {
		// 		raw_value = slice_val[indice]
		// 	} else {
		// 		raw_value = nil
		// 	}
		// case []float64:
		// 	if indice < len(slice_val) {
		// 		raw_value = slice_val[indice]
		// 	} else {
		// 		raw_value = nil
		// 	}
		// default:
		// 	raw_value = nil
		// }
	}
	return res_value
}

func buildAttrWithIndice(symtab map[string]any, raw_value any, var_name, indice_str string) (any, error) {
	vDst := reflect.ValueOf(raw_value)
	if vDst.Kind() == reflect.Map {
		if indice_str[0] == '$' {
			tmp_name, _ := extract_var_name(indice_str[1:])
			raw_ind, err := getVar(symtab, tmp_name)
			if err != nil {
				return nil, err
			} else {
				indice_str = cast.ToString(raw_ind)
			}
		}
		raw_value = getMapKey(raw_value, indice_str)
		if raw_value == nil {
			return nil, newVarError(error_var_mapkey_not_found,
				fmt.Sprintf("key '%s' not found in %s map", indice_str, var_name))
		}
	} else if vDst.Kind() == reflect.Slice {
		var indice int

		if indice_str[0] == '$' {
			tmp_name, _ := extract_var_name(indice_str[1:])
			raw_ind, err := getVar(symtab, tmp_name)
			if err != nil {
				return nil, err
			} else {
				indice = cast.ToInt(raw_ind)
			}
		} else {
			if i_value, err := strconv.ParseInt(indice_str, 10, 0); err != nil {
				indice = 0
			} else {
				indice = int(i_value)
			}
		}
		raw_value = getSliceIndice(raw_value, indice)
		if raw_value == nil {
			return nil, newVarError(error_var_sliceindice_not_found,
				fmt.Sprintf("indice '%s' not found in %s array", indice_str, var_name))
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
	error_var_not_found             = iota + 1
	error_var_invalid_type          = iota
	error_var_invalid_template      = iota
	error_var_invalid_json_output   = iota
	error_var_mapkey_not_found      = iota
	error_var_sliceindice_not_found = iota
)

type varError struct {
	code    int
	message string
}

type VarError interface {
	Code() int
	Error() string
}

// func (e *VarError) CodeToText() string {
// 	var res string
// 	switch e.Code {
// 	case error_var_not_found:
// 		res = "error var not found"
// 	}
// 	return res
// }

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
func getVar(symtab map[string]any, attr string) (any, error) {
	var err error

	tmp_symtab := symtab
	// split the attr string into parts: attr1.attr[0].attr
	if attr[0] == '.' {
		attr = attr[1:]
	}
	vars := strings.Split(attr, ".")
	lenattr := len(vars) - 1
	for idx, var_name := range vars {
		indice_str := ""
		// check if attr refers to an another variable name $X.[...].$var_name
		if var_name[0] == '$' {
			tmp_name, pos := extract_var_name(var_name[1:])
			raw_value, err := getVar(symtab, tmp_name)
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
		// check if component contains indice pos: attr[x]
		if pos := strings.Index(var_name, "["); pos != -1 {
			pos2 := strings.Index(var_name, "]")
			indice_str = var_name[pos+1 : pos2]
			var_name = var_name[0:pos]
			// remove enclosing string separators if any found
			indice_str = strings.Trim(indice_str, "'`\"")
		}
		// try to find attribute name as key name of map element
		if raw_value, ok := tmp_symtab[var_name]; ok {
			if value, ok := raw_value.(*Field); ok {
				raw_value, err = value.GetValueObject(symtab)
				if err != nil {
					return nil, err
				}
			}

			// special case attribute name contains '[indice_str]'
			if indice_str != "" {
				raw_value, err = buildAttrWithIndice(symtab, raw_value, var_name, indice_str)
				if err != nil {
					return nil, err
				}
				// vDst := reflect.ValueOf(raw_value)
				// if vDst.Kind() == reflect.Map {
				// 	raw_value = getMapKey(raw_value, indice_str)
				// 	if raw_value == nil {
				// 		return nil, fmt.Errorf("key '%s' not found in %s map", indice_str, var_name)
				// 	}
				// } else if vDst.Kind() == reflect.Slice {
				// 	var indice int
				// 	if i_value, err := strconv.ParseInt(indice_str, 10, 0); err != nil {
				// 		indice = 0
				// 	} else {
				// 		indice = int(i_value)
				// 	}
				// 	raw_value = getSliceIndice(raw_value, indice)
				// 	if raw_value == nil {
				// 		return nil, fmt.Errorf("indice '%s' not found in %s array", indice_str, var_name)
				// 	}
				// }
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
				// switch cur_value := raw_value.(type) {
				// case map[string]string:
				// 	mAmy := make(map[string]any)
				// 	for k, v := range cur_value {
				// 		mAmy[k] = v
				// 	}
				// 	tmp_symtab = mAmy

				// case map[string]any:
				// 	tmp_symtab = cur_value
				// default:
				// 	err = fmt.Errorf("can't set attr: '%s' has invalid type", var_name)
				// }
			} else {
				// if value, ok := raw_value.(*Field); ok {
				// 	raw_value, err = value.GetValueObject(symtab, with_raw_name)
				// 	if err != nil {
				// 		return nil, err
				// 	}
				// 	if indice_str != "" {
				// 		raw_value, err = buildAttrWithIndice(raw_value, var_name, indice_str)
				// 		if err != nil {
				// 			return nil, err
				// 		}
				// 		// raw_value = getSliceIndice(raw_value, indice)
				// 	}
				// }
				return raw_value, err
			}
			// }
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
	// item map[string]interface{}) ([]any, error) {
	item any,
) (res any, err error) {
	res_slice := make([]any, 0)

	if f == nil {
		return res_slice, nil
	}

	if f.tmpl != nil {
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
	} else {
		if f.vartype {
			if symtab, ok := item.(map[string]any); ok {
				data, err := getVar(symtab, f.raw)
				if err != nil {
					return res_slice, err
				}

				return data, nil
			} else {
				return nil, nil
			}
		}
		// else it is a simple string `value`
		return RawGetValueString(f.raw), nil
	}
}

func (f *Field) String() string {
	if f == nil {
		return ""
	}

	if f.tmpl != nil {
		// f.tmpl.
		return f.tmpl.Tree.Root.String()
	} else {
		if f.vartype {
			return "$" + f.raw
		}
		return f.raw
	}
}

func (f *Field) MarshalText() (text []byte, err error) {

	return []byte(f.String()), nil
}

func (f *Field) AddDefaultTemplate(customTemplate *exporterTemplate) error {
	if f.tmpl != nil && customTemplate != nil {
		if tmpl, err := AddDefaultTemplate(f.tmpl, customTemplate); err != nil {
			f.tmpl = tmpl
		} else {
			return err
		}
	}
	return nil
}

func AddDefaultTemplate(dest_tmpl *exporterTemplate, customTemplate *exporterTemplate) (*exporterTemplate, error) {
	if dest_tmpl != nil && customTemplate != nil {
		cc_tmpl, err := ((*ttemplate.Template)(customTemplate)).Clone()
		if err != nil {
			return nil, fmt.Errorf("field template clone for %s is invalid: %s", ((*ttemplate.Template)(customTemplate)).Name(), err)
		}
		for _, tmpl := range cc_tmpl.Templates() {
			name := tmpl.Name()
			if name == "default" {
				continue
			}

			_, err = ((*ttemplate.Template)(dest_tmpl)).AddParseTree(tmpl.Name(), tmpl.Tree)
			if err != nil {
				return nil, fmt.Errorf("field template %s is invalid: %s", tmpl.Name(), err)
			}
		}
	}
	return dest_tmpl, nil
}

type ResultElement struct {
	raw any
}

func (r *ResultElement) GetSlice(field string) ([]any, bool) {
	var myslice []any
	var found bool

	// 	vSrc := reflect.ValueOf(r.raw)
	// 	if vSrc.Kind() == reflect.Map {
	// 		n_slice := make([]any, 1)
	// 		n_slice[0] = raw
	// 		myslice = n_slice
	// 		found = true
	// raw_value = getMapKey(raw_value, indice_str)
	// 		if raw_value == nil {
	// 			return nil, fmt.Errorf("key '%s' not found in %s map", indice_str, var_name)
	// 		}
	// 	} else if vSrc.Kind() == reflect.Slice {
	// 		// if src is a slice,
	// 		myslice = make([]any, vSrc.Len())
	// 		for idx := range vSrc.Len() {
	// 			myslice[idx] = vSrc.Index(idx).Interface()
	// 		}
	// 		found = true
	// 	}
	if r_type, ok := r.raw.(map[string]any); ok {

		if myvar, ok := r_type[field]; ok {
			switch curval := myvar.(type) {
			case []any:
				myslice = curval
				found = true
			case map[string]any:
				n_slice := make([]any, 1)
				n_slice[0] = myvar
				myslice = n_slice
				found = true

			}
		}
	}
	return myslice, found
}
