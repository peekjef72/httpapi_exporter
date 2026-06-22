package main

import (
	"fmt"
	"log/slog"
	"reflect"
	"strings"

	"github.com/spf13/cast"
)

type Variable struct {
	vartype    int
	raw        string
	attributes []*Variable
}
type VariableStack []Variable

const (
	vartype_var = iota
	vartype_attribute
)

func ParseVariables(expression string) (*Variable, error) {
	vars, _, err := parseVariables(expression, 0)
	return vars, err
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

func parseVariables(expression string, level int) (*Variable, int, error) {
	var current *Variable
	var currentVar strings.Builder

	// only allow $var or [val] inside recursion
	if level == 0 && expression[0] != '$' {
		return nil, 0, fmt.Errorf("invalid starting char not $ at pos 0")
	}
	current = &Variable{}
	isAttribute := false

	if level == 0 {
		current.vartype = vartype_var
	} else {
		current.vartype = vartype_attribute
		// isAttribute = true
	}

	startup := true
	element_end := false
	begin_double_quote := false
	begin_simple_quote := false

	for i := 0; i < len(expression); i++ {
		char := rune(expression[i])

		switch char {
		case '"':
			// start new string
			if !begin_double_quote {
				if currentVar.Len() > 0 {
					return nil, 0, fmt.Errorf("invalid char '\"' in lexeme definition at pos %d", i)
				}
				begin_double_quote = true
				element_end = false
				startup = false
				continue
			} else {
				// end new string
				begin_double_quote = false
				element_end = true

			}
		case '\'':
			// start new string
			if !begin_simple_quote {
				if currentVar.Len() > 0 {
					return nil, 0, fmt.Errorf("invalid char '\"' in lexeme definition at pos %d", i)
				}
				begin_simple_quote = true
				startup = false
				continue
			} else {
				// end new string
				begin_simple_quote = false
				element_end = true
			}

		case '$':
			if begin_double_quote || begin_simple_quote {
				currentVar.WriteRune(char)
				continue
			}
			if element_end {
				return nil, 0, fmt.Errorf("invalid char '$' after string in definition at pos %d", i)
			}
			// found a '$': it is a new var
			// check current "stack" type and status: $ is allowed only at startup of word
			if !startup {
				return nil, 0, fmt.Errorf("invalid char '$' in definition at pos %d", i)
			}
			// check obsolete special case : $var.${var2}
			if isAttribute && currentVar.Len() == 0 {
				name, pos := extract_var_name(expression[i:])
				if name[0] != '$' {
					name = "$" + name
				}
				if sub_var, _, err := parseVariables(name, level+1); err == nil {
					// dst_var.attributes = append(dst_var.attributes, sub_var)
					current.attributes = append(current.attributes, sub_var)
				}
				i += pos
			} else {
				isAttribute = false
				current.vartype = vartype_var
				startup = false
				// reduce current element and start a new var
				if currentVar.Len() > 0 {
					current.raw = strings.TrimSpace(currentVar.String())
					currentVar.Reset()
					startup = true
					element_end = false
				}
			}
			// eat up the $ (remove from stack)

		case '.':
			if begin_double_quote || begin_simple_quote {
				currentVar.WriteRune(char)
				continue
			}
			// if element_end {
			// 	return nil, 0, fmt.Errorf("invalid char '.' after string in definition at pos %d", i)
			// }
			// found a '.': it is a new attribute; reduce previous content
			// $var.<-attr or $var.attr1.<-attr2 : collect  'var' as var name or 'attr1' as attribute element
			// if level > 0 .attr is only authorized if current has type var $var[attr1.attr2] ko : only $var[$var2.attr]
			if level > 0 && current.vartype != vartype_var {
				return nil, 0, fmt.Errorf("invalid char '.' in definition at pos %d for something not a variable", i)
			}
			if currentVar.Len() > 0 {
				if !isAttribute {
					current.raw = strings.TrimSpace(currentVar.String())
				} else {
					attribute := &Variable{
						raw:     strings.TrimSpace(currentVar.String()),
						vartype: vartype_attribute,
					}
					current.attributes = append(current.attributes, attribute)
				}
				currentVar.Reset()
				startup = true
				element_end = false
			}
			isAttribute = true

		case '[':
			if begin_double_quote || begin_simple_quote {
				currentVar.WriteRune(char)
				continue
			}
			// found a '[': it is a new name or var; reduce previous content
			// $var[<-sub] or $var.attr[<-sub] : get 'var' as var name or 'attr' as attribute element
			// var dst_var *Variable
			if currentVar.Len() > 0 {
				if !isAttribute {
					current.raw = strings.TrimSpace(currentVar.String())
					// dst_var = current
				} else {
					attribute := &Variable{
						raw:     strings.TrimSpace(currentVar.String()),
						vartype: vartype_attribute,
					}
					current.attributes = append(current.attributes, attribute)
					// dst_var = attribute
				}
				currentVar.Reset()
				// } else {
				// 	dst_var = current
			}
			if sub_var, pos, err := parseVariables(expression[i+1:], level+1); err == nil {
				// dst_var.attributes = append(dst_var.attributes, sub_var)
				current.attributes = append(current.attributes, sub_var)
				i += pos
			} else {
				return nil, pos, err
			}
			startup = true
			element_end = false

		case ']':
			if begin_double_quote || begin_simple_quote {
				currentVar.WriteRune(char)
				continue
			}
			// found a closing bracket ]: reduce previous var and return
			// check if level is enough
			if level < 0 {
				return nil, 0, fmt.Errorf("invalid char ']' in definition at pos: %d", i)
			}
			// {level 0} '$var[' {level 1}0]<-sub] or {level 1}$var.attr]<-sub : get '0' as var name or 'attr' as attribute element
			if currentVar.Len() > 0 {
				if !isAttribute {
					current.raw = strings.TrimSpace(currentVar.String())
				} else {
					attribute := &Variable{
						raw:     strings.TrimSpace(currentVar.String()),
						vartype: vartype_attribute,
					}
					current.attributes = append(current.attributes, attribute)
				}
				currentVar.Reset()
				element_end = false
			} else if len(current.attributes) == 0 {
				return nil, i + 1, fmt.Errorf("empty attribute in definition at pos: %d", i)
			}
			return current, i + 1, nil

		default:
			if element_end {
				return nil, 0, fmt.Errorf("invalid char '%c' after string in definition at pos %d", char, i)
			}
			// add rune into the current var content
			startup = false
			currentVar.WriteRune(char)
		}
	}

	if currentVar.Len() > 0 {
		if begin_double_quote || begin_simple_quote {
			return nil, 0, fmt.Errorf("string not terminated")
		}
		// at level 0 "$var" => get $var / at other level  'index' or 'attr' => get 'index'
		if !isAttribute {
			current.raw = strings.TrimSpace(currentVar.String())
		} else {
			// $var.attr => get attr as attribute
			attribute := &Variable{
				raw:     strings.TrimSpace(currentVar.String()),
				vartype: vartype_attribute,
			}
			current.attributes = append(current.attributes, attribute)
		}
	}

	return current, len(expression), nil
}

func (v *Variable) String() string {
	var output strings.Builder
	v.toStringFragment(&output)
	return output.String()
}

func (v *Variable) toStringFragment(output *strings.Builder) {
	if v.vartype == vartype_var {
		output.WriteRune('$')
	}
	output.WriteString(v.raw)

	for _, attr := range v.attributes {
		if len(attr.attributes) > 0 {
			output.WriteRune('[')
			attr.toStringFragment(output)
			output.WriteRune(']')
		} else {
			output.WriteRune('.')
			output.WriteString(attr.raw)
		}
	}
}

// ***************************************************************************************
func (v *Variable) GetVar(
	symtab map[string]any,
	attribute_symtab map[string]any,
	logger *slog.Logger,
) (any, error) {
	var (
		err       error
		ok        bool
		raw_value any
	)

	tmp_symtab := symtab
	if attribute_symtab != nil {
		tmp_symtab = attribute_symtab
	}

	if v.vartype == vartype_var {
		var_name := v.raw
		if raw_value, ok = symtab[v.raw]; ok {
			if value, ok := raw_value.(*Field); ok {
				raw_value, err = value.GetValueObject(symtab, logger)
				if err != nil {
					return nil, err
				}
			}
			if len(v.attributes) == 0 {
				return raw_value, nil
			}

			for _, attribute := range v.attributes {

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
					// build map for next attribute
					tmp_symtab = mAny
					var (
						raw_attribute any
					)
					if raw_attribute, err = attribute.GetVar(symtab, tmp_symtab, logger); err != nil {
						return nil, err
					}
					attribute_name := RawGetValueString(raw_attribute)
					raw_value = getMapKey(tmp_symtab, attribute_name)
					if raw_value == nil {
						return nil, newVarError(error_var_mapkey_not_found,
							fmt.Sprintf("key '%s' not found in %s map", attribute_name, var_name))
					}
				} else if vDst.Kind() == reflect.Slice {
					var (
						raw_index any
						index     int
					)
					if raw_index, err = attribute.GetVar(symtab, tmp_symtab, logger); err != nil {
						return nil, err
					}
					index, err = rawGetValueInt64(raw_index)
					if err != nil {
						return nil, newVarError(error_var_invalid_type,
							fmt.Sprintf("attribute '%s' is not number for slice index", attribute.String()))
					}
					raw_value = getSliceIndex(raw_value, index)
					if raw_value == nil {
						return nil, newVarError(error_var_sliceindex_not_found,
							fmt.Sprintf("index '%d' not found in %s array", index, var_name))
					}
				} else {
					return nil, newVarError(error_var_invalid_type,
						fmt.Sprintf("attribute '%s' is neither of 'map' type nor 'slice' type", var_name))
				}
				if raw_value != nil {
					if value, ok := raw_value.(*Field); ok {
						raw_value, err = value.GetValueObject(symtab, logger)
						if err != nil {
							return nil, err
						}
					}
				}
			}
		} else {
			err = newVarError(error_var_not_found,
				fmt.Sprintf("%s not found", var_name))
			tmp_symtab = nil
		}
	} else {
		raw_value = v.raw
	}

	return raw_value, nil
}

// obtain int64 from a var of any type
func rawGetValueInt64(current_val any) (num_value int, err error) {
	if current_val != nil {
		num_value, err = cast.ToIntE(current_val)
	} else {
		num_value = 0
	}
	return num_value, err
}
