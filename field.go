package main

import (
	"encoding/json"
	"fmt"
	"html"

	// "regexp"

	"strconv"
	ttemplate "text/template"

	"strings"
)

// var base60float = regexp.MustCompile(`^[-+]?[0-9][0-9_]*(?::[0-5]?[0-9])+(?:\.[0-9_]*)?$`)

type exporterTemplate ttemplate.Template

func (tmpl *exporterTemplate) MarshalText() (text []byte, err error) {

	return []byte(tmpl.Tree.Root.String()), nil
}

type Field struct {
	raw  string
	tmpl *exporterTemplate
}

// create a new key or value Field that can be a GO template
func NewField(name string, customTemplate *exporterTemplate) (*Field, error) {
	var tmpl *ttemplate.Template
	var err error

	if strings.Contains(name, "{") {
		if customTemplate != nil {
			ptr := (*ttemplate.Template)(customTemplate)
			tmpl, err = ptr.Clone()
			if err != nil {
				return nil, fmt.Errorf("field template clone for %s is invalid: %s", name, err)
			}
		} else {
			// tmpl = template.New("field").Funcs(sprig.FuncMap())
			tmpl = ttemplate.New("field").Funcs(mymap())
		}
		tmpl, err = tmpl.Parse(name)
		if err != nil {
			return nil, fmt.Errorf("field template %s is invalid: %s", name, err)
		}
	}
	return &Field{
		raw:  name,
		tmpl: (*exporterTemplate)(tmpl),
	}, nil
}

// obtain float64 from a var of any type
func RawGetValueFloat(curval any) float64 {
	var f_value float64
	var err error
	if curval == nil {
		return 0.0
	}

	// it is a raw value not a template look in "item"
	switch curval := curval.(type) {
	case int64:
		f_value = float64(curval)
	case float64:
		f_value = curval
	case string:
		if f_value, err = strconv.ParseFloat(strings.Trim(curval, "\r\n "), 64); err != nil {
			f_value = 0
		}
	default:
		f_value = 0
		// value := row[v].(float64)
	}
	return f_value
}

// obtain string from a var of any type
func RawGetValueString(curval any) string {
	res := ""
	if curval == nil {
		return res
	}

	switch curval := curval.(type) {
	case string:
		res = curval
	// case map[any]any:
	// 	bytes_res, err := json.MarshalIndent(curval, "", "")
	// 	if err != nil {
	// 		fmt.Printf("error: %s\n", string(err.Error()))
	// 	}
	// 	res = string(bytes_res)
	default:
		res = fmt.Sprintf("%v", curval)
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
	sub map[string]string,
	check_item bool) (res string, err error) {
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
					err = fmt.Errorf("panic in GetValueString template with undefined error")
				}
				res = ""
			}
		}()
		tmp_res := new(strings.Builder)
		err = ((*ttemplate.Template)(f.tmpl)).Execute(tmp_res, &item)
		if err != nil {
			return "", err
		}
		// obtain final string from builder
		tmp := tmp_res.String()
		// remove before and after blank chars
		tmp = strings.Trim(tmp, "\r\n ")
		// unescape string
		return html.UnescapeString(tmp), nil
	} else {
		val := f.raw
		// check if there is a transformation value in sub[stitution] map
		if sub != nil {
			if _, ok := sub[val]; ok {
				val = sub[val]
			}
		}
		if check_item {
			if curval, ok := item[val]; ok {
				return RawGetValueString(curval), nil
			}
		}
		return RawGetValueString(val), nil
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
					err = fmt.Errorf("panic in GetValueFloat template with undefined error")
				}
				res = 0
			}
		}()

		tmp_res := new(strings.Builder)
		err := ((*ttemplate.Template)(f.tmpl)).Execute(tmp_res, &item)
		if err != nil {
			return 0, err
		}
		str_value = html.UnescapeString(tmp_res.String())
	} else {
		val := f.raw
		// check if value exists in symbol table
		if curval, ok := item[val]; ok {
			str_value = curval
		} else {
			str_value = val
		}
	}
	return RawGetValueFloat(str_value), nil
}

func (f *Field) GetValueObject(
	// item map[string]interface{}) ([]any, error) {
	item any) (res any, err error) {
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
					err = fmt.Errorf("panic in GetValueObject template with undefined error")
				}
				res = res_slice
			}
		}()
		tmp_res := new(strings.Builder)
		err := ((*ttemplate.Template)(f.tmpl)).Execute(tmp_res, &item)
		if err != nil {
			return res_slice, err
		}
		var data any
		json_obj := tmp_res.String()
		if json_obj == "<no value>" || json_obj == "" || json_obj == "null" {
			data = ""
		} else {
			// json_obj = strings.ReplaceAll(json_obj, "&#34;", "\"")
			json_obj = html.UnescapeString(json_obj)
			// json_obj = strings.TrimSuffix(json_obj, "\n")
			// fmt.Println(json_obj)
			err = json.Unmarshal([]byte(json_obj), &data)
			if err != nil {
				if _, ok := err.(*json.SyntaxError); ok {
					// invalid character 'X' in literal true
					return json_obj, nil
				}
				return res_slice, err
			}
		}
		return data, nil
	} else {
		datas := &ResultElement{
			raw: item,
		}
		if data, found := datas.GetSlice(f.raw); found {
			return data, nil
		} else {
			return nil, nil
		}
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
			// myvart := reflect.ValueOf(myvar).Kind()
			// if myvart == reflect.Slice {
			// 	myslice = myvar.([]interface{})
			// 	// for i, myvar := range myslice {
			// 	// 	myvart := reflect.ValueOf(myvar).Kind()
			// 	// 	if myvart == reflect.Map {
			// 	// 		mymap := make(map[string]interface{})
			// 	// 		for k, v := range myvar.(map[string]interface{}) {
			// 	// 			fmt.Printf("k: %s, v %+v\n", k, v)
			// 	// 		}
			// 	// 	}
			// 	// }
			// } else if myvart == reflect.Map {
			// 	n_slice := make([]interface{}, 1)
			// 	n_slice[0] = myvar
			// 	myslice = n_slice
			// }
		}
	}
	return myslice, found
}

// func (r *ResultElement) GetMap(field string) map[string]interface{} {
// 	var mymap map[string]interface{}
// 	myvart := reflect.ValueOf(r.raw).Kind()
// 	if myvart != reflect.Map {
// 		mymap = nil
// 	}
// 	return mymap
// }
