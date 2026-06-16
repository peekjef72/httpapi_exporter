// cSpell:ignore stretchr, mymap, constmap

package main

import (
	"fmt"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	symtab map[string]any
	logger *slog.Logger
)

func initTest() {
	symtab = make(map[string]any)
	logger = &slog.Logger{}
}

// doc
//
// parse $var
//
// => variable 'var' and no attribute
func TestParseVar(t *testing.T) {
	initTest()

	test := "$var"
	if variable, err := ParseVariables(test); err != nil {
		t.Errorf(`TestParseVar("%s") error: %s`, test, err.Error())
	} else {
		assert.True(t, variable.raw == "var",
			fmt.Sprintf(`TestParseVar("%s") value differ: %s`, variable.raw, "var"),
		)
		assert.True(t, variable.vartype == vartype_var,
			fmt.Sprintf(`TestParseVar("%s") vartype differ: %d`, variable.raw, variable.vartype),
		)
		assert.True(t, len(variable.attributes) == 0,
			fmt.Sprintf(`TestParseVar("%s") attributes count differ - must be 0: %v`, variable.raw, variable.attributes),
		)
	}
}

// doc2
//
// parse $var.attr1
//
// => variable 'var' and 1 attribute 'attr1'
func TestParseVar2(t *testing.T) {
	initTest()

	test := "$var.attr1"
	if variable, err := ParseVariables(test); err != nil {
		t.Errorf(`TestParseVar("%s") error: %s`, test, err.Error())
	} else {
		assert.True(t, variable.raw == "var",
			fmt.Sprintf(`TestParseVar("%s") value differ: %s`, variable.raw, "var"),
		)
		assert.True(t, len(variable.attributes) == 1,
			fmt.Sprintf(`TestParseVar("%s") attributes count differ - must be 1: %v`, variable.raw, variable.attributes),
		)
		assert.True(t, variable.attributes[0].raw == "attr1",
			fmt.Sprintf(`TestParseVar("%s") attribute value differ: %s`, variable.attributes[0].raw, "attr1"),
		)
		assert.True(t, variable.attributes[0].vartype == vartype_attribute,
			fmt.Sprintf(`TestParseVar("%s") vartype differ: %d`, variable.raw, variable.attributes[0].vartype),
		)
	}
}

// doc
//
// parse $var['attr1'] idem $var.attr1
//
// => variable 'var' 1 attribute 'attr1'
func TestParseVar3(t *testing.T) {
	initTest()

	test := "$var['attr1]"
	if variable, err := ParseVariables(test); err != nil {
		t.Errorf(`TestParseVar("%s") error: %s`, test, err.Error())
	} else {
		assert.True(t, variable.raw == "var",
			fmt.Sprintf(`TestParseVar("%s") value differ: %s`, variable.raw, "var"),
		)
		assert.True(t, len(variable.attributes) == 1,
			fmt.Sprintf(`TestParseVar("%s") attributes count differ - must be 1: %v`, variable.raw, variable.attributes),
		)
		assert.True(t, variable.attributes[0].raw == "attr1",
			fmt.Sprintf(`TestParseVar("%s") attribute value differ: %s`, variable.attributes[0].raw, "attr1"),
		)
	}
}

// doc4
//
// parse $var[$var2.attr]
//
// => variable 'var' 1 attribute 'var2'
//
//	var2 type variable with 1 attribute attr
func TestParseVar4(t *testing.T) {
	initTest()

	test := "$var[$var2.attr]"
	if variable, err := ParseVariables(test); err != nil {
		t.Errorf(`TestParseVar("%s") error: %s`, test, err.Error())
	} else {
		assert.True(t, variable.raw == "var",
			fmt.Sprintf(`TestParseVar("%s") value differ: %s`, variable.raw, "var"),
		)
		assert.True(t, len(variable.attributes) == 1,
			fmt.Sprintf(`TestParseVar("%s") attributes count differ - must be 1: %v`, variable.raw, variable.attributes),
		)
		sub_attr := variable.attributes[0]

		assert.True(t, sub_attr.raw == "var2",
			fmt.Sprintf(`TestParseVar("%s") attribute value differ: %s`, variable.attributes[0].raw, "var2"),
		)
		assert.True(t, sub_attr.vartype == vartype_var,
			fmt.Sprintf(`TestParseVar("%s") vartype differ: %d`, sub_attr.raw, sub_attr.vartype),
		)
		assert.True(t, len(sub_attr.attributes) == 1,
			fmt.Sprintf(`TestParseVar("%s") attributes count differ - must be 1: %v`, sub_attr.raw, sub_attr.attributes),
		)
		assert.True(t, sub_attr.attributes[0].raw == "attr",
			fmt.Sprintf(`TestParseVar("%s") attribute value differ: %s`, sub_attr.attributes[0].raw, "attr"),
		)
		assert.True(t, sub_attr.attributes[0].vartype == vartype_attribute,
			fmt.Sprintf(`TestParseVar("%s") vartype differ: %d`, sub_attr.raw, sub_attr.attributes[0].vartype),
		)
	}
}

// doc5
//
// parse $var[$var2.attr].attr2
//
// => variable 'var' 2 attributes, 'var2.XXX', attr2
//
// - var2 type variable with 1 attribute attr
//
// - attr2 type attr without attribute
func TestParseVar5(t *testing.T) {
	initTest()

	test := "$var[$var2.attr].attr2"
	if variable, err := ParseVariables(test); err != nil {
		t.Errorf(`TestParseVar("%s") error: %s`, test, err.Error())
	} else {
		assert.True(t, variable.raw == "var",
			fmt.Sprintf(`TestParseVar("%s") value differ: %s`, variable.raw, "var"),
		)
		assert.True(t, len(variable.attributes) == 2,
			fmt.Sprintf(`TestParseVar("%s") attributes count differ - must be 2: %v`, variable.raw, variable.attributes),
		)

		sub_attr := variable.attributes[0]

		assert.True(t, sub_attr.raw == "var2",
			fmt.Sprintf(`TestParseVar("%s") attribute value differ: %s`, variable.attributes[0].raw, "var2"),
		)
		assert.True(t, sub_attr.vartype == vartype_var,
			fmt.Sprintf(`TestParseVar("%s") vartype differ: %d`, sub_attr.raw, sub_attr.vartype),
		)
		assert.True(t, len(sub_attr.attributes) == 1,
			fmt.Sprintf(`TestParseVar("%s") attributes count differ - must be 1: %v`, sub_attr.raw, sub_attr.attributes),
		)
		assert.True(t, sub_attr.attributes[0].raw == "attr",
			fmt.Sprintf(`TestParseVar("%s") attribute value differ: %s`, sub_attr.attributes[0].raw, "attr"),
		)
		assert.True(t, sub_attr.attributes[0].vartype == vartype_attribute,
			fmt.Sprintf(`TestParseVar("%s") vartype differ: %d`, sub_attr.raw, sub_attr.attributes[0].vartype),
		)

		sub_attr = variable.attributes[1]
		assert.True(t, sub_attr.raw == "attr2",
			fmt.Sprintf(`TestParseVar("%s") attribute2 value differ: %s`, variable.attributes[0].raw, "attr2"),
		)
		assert.True(t, sub_attr.vartype == vartype_attribute,
			fmt.Sprintf(`TestParseVar("%s") vartype differ: %d`, sub_attr.raw, sub_attr.vartype),
		)
		assert.True(t, len(sub_attr.attributes) == 0,
			fmt.Sprintf(`TestParseVar("%s") attributes count differ - must be 0: %v`, sub_attr.raw, sub_attr.attributes),
		)

	}
}

// doc6
//
// parse $var[$var2.attr].attr2['attr3']
//
// => variable 'var' 3 attributes, 'var2.XXX', attr2, attr3
//
// - var2 type variable with 1 attribute attr
//
// - attr2 type attr without attribute
//
// - attr3 type attr without attribute
func TestParseVar6(t *testing.T) {
	initTest()

	test := "$var[$var2.attr].attr2['attr3']"
	if variable, err := ParseVariables(test); err != nil {
		t.Errorf(`TestParseVar("%s") error: %s`, test, err.Error())
	} else {
		assert.True(t, variable.raw == "var",
			fmt.Sprintf(`TestParseVar("%s") value differ: %s`, variable.raw, "var"),
		)
		assert.True(t, len(variable.attributes) == 3,
			fmt.Sprintf(`TestParseVar("%s") attributes count differ - must be 3: %v`, variable.raw, variable.attributes),
		)
		var sub_attr *Variable

		if len(variable.attributes) > 0 {
			sub_attr = variable.attributes[0]

			assert.True(t, sub_attr.raw == "var2",
				fmt.Sprintf(`TestParseVar("%s") attribute value differ: %s`, variable.attributes[0].raw, "var2"),
			)
			assert.True(t, sub_attr.vartype == vartype_var,
				fmt.Sprintf(`TestParseVar("%s") vartype differ: %d`, sub_attr.raw, sub_attr.vartype),
			)
			assert.True(t, len(sub_attr.attributes) == 1,
				fmt.Sprintf(`TestParseVar("%s") attributes count differ - must be 1: %v`, sub_attr.raw, sub_attr.attributes),
			)
			assert.True(t, sub_attr.attributes[0].raw == "attr",
				fmt.Sprintf(`TestParseVar("%s") attribute value differ: %s`, sub_attr.attributes[0].raw, "attr"),
			)
			assert.True(t, sub_attr.attributes[0].vartype == vartype_attribute,
				fmt.Sprintf(`TestParseVar("%s") vartype differ: %d`, sub_attr.raw, sub_attr.attributes[0].vartype),
			)
		}

		if len(variable.attributes) > 1 {
			sub_attr = variable.attributes[1]
			assert.True(t, sub_attr.raw == "attr2",
				fmt.Sprintf(`TestParseVar("%s") attribute2 value differ: %s`, sub_attr.raw, "attr2"),
			)
			assert.True(t, sub_attr.vartype == vartype_attribute,
				fmt.Sprintf(`TestParseVar("%s") vartype differ: %d`, sub_attr.raw, sub_attr.vartype),
			)
			assert.True(t, len(sub_attr.attributes) == 0,
				fmt.Sprintf(`TestParseVar("%s") attributes count differ - must be 0: %v`, sub_attr.raw, sub_attr.attributes),
			)
		}
		if len(variable.attributes) > 2 {

			sub_attr = variable.attributes[2]
			assert.True(t, sub_attr.raw == "attr3",
				fmt.Sprintf(`TestParseVar("%s") attribute3 value differ: %s`, sub_attr.raw, "attr2"),
			)
			assert.True(t, sub_attr.vartype == vartype_attribute,
				fmt.Sprintf(`TestParseVar("%s") vartype differ: %d`, sub_attr.raw, sub_attr.vartype),
			)
			assert.True(t, len(sub_attr.attributes) == 0,
				fmt.Sprintf(`TestParseVar("%s") attributes count differ - must be 0: %v`, sub_attr.raw, sub_attr.attributes),
			)
		}
	}
}

// doc7
// parse "$var1[$var2.attr1.attr2[$var3.attr3.attr4.attr5]].attr6.attr7"
//
// => variable 'var' 3 attributes, 'var2.XXX', attr6, attr7
//
// - var2 type variable with 3 attributes attr1, attr2, var3[xxx]
//
// - attr6 type attr without attribute
//
// - attr7 type attr without attribute
func TestParseVar7(t *testing.T) {
	initTest()

	test := "$var1[$var2.attr1.attr2[$var3.attr3.attr4.attr5]].attr6.attr7"
	if variable, err := ParseVariables(test); err != nil {
		t.Errorf(`TestParseVar("%s") error: %s`, test, err.Error())
	} else {
		assert.True(t, variable.raw == "var1",
			fmt.Sprintf(`TestParseVar("%s") value differ: %s`, variable.raw, "var1"),
		)
		assert.True(t, len(variable.attributes) == 3,
			fmt.Sprintf(`TestParseVar("%s") attributes count differ - must be 3: %v`, variable.raw, variable.attributes),
		)
		var sub_attr *Variable

		if len(variable.attributes) > 0 {
			sub_attr = variable.attributes[0]

			assert.True(t, sub_attr.raw == "var2",
				fmt.Sprintf(`TestParseVar("%s") attribute value differ: %s`, variable.attributes[0].raw, "var2"),
			)
			assert.True(t, sub_attr.vartype == vartype_var,
				fmt.Sprintf(`TestParseVar("%s") vartype differ: %d`, sub_attr.raw, sub_attr.vartype),
			)
			assert.True(t, len(sub_attr.attributes) == 3,
				fmt.Sprintf(`TestParseVar("%s") attributes count differ - must be 3: %v`, sub_attr.raw, sub_attr.attributes),
			)
			assert.True(t, sub_attr.attributes[0].raw == "attr1",
				fmt.Sprintf(`TestParseVar("%s") attribute value differ: %s`, sub_attr.attributes[0].raw, "attr1"),
			)
			assert.True(t, sub_attr.attributes[0].vartype == vartype_attribute,
				fmt.Sprintf(`TestParseVar("%s") vartype differ: %d`, sub_attr.raw, sub_attr.attributes[0].vartype),
			)

			if len(sub_attr.attributes) > 2 {
				sub_attr = sub_attr.attributes[2]
				assert.True(t, sub_attr.raw == "var3",
					fmt.Sprintf(`TestParseVar("%s") attribute value differ: %s`, sub_attr.raw, "var3"),
				)
				assert.True(t, sub_attr.vartype == vartype_var,
					fmt.Sprintf(`TestParseVar("%s") vartype differ: %d`, sub_attr.raw, sub_attr.vartype),
				)
				assert.True(t, len(sub_attr.attributes) == 3,
					fmt.Sprintf(`TestParseVar("%s") attributes count differ - must be 3: %v`, sub_attr.raw, sub_attr.attributes),
				)
			}
		}

		if len(variable.attributes) > 2 {

			sub_attr = variable.attributes[2]
			assert.True(t, sub_attr.raw == "attr7",
				fmt.Sprintf(`TestParseVar("%s") attribute4 value differ: %s`, sub_attr.raw, "attr7"),
			)
			assert.True(t, sub_attr.vartype == vartype_attribute,
				fmt.Sprintf(`TestParseVar("%s") vartype differ: %d`, sub_attr.raw, sub_attr.vartype),
			)
			assert.True(t, len(sub_attr.attributes) == 0,
				fmt.Sprintf(`TestParseVar("%s") attributes count differ - must be 0: %v`, sub_attr.raw, sub_attr.attributes),
			)
		}

	}
}

// doc8
//
// parse $var.$attr1
//
// => variable 'var' and 1 attribute 'attr1' of typ vartype_var
func TestParseVar8(t *testing.T) {
	initTest()

	test := "$var.$attr1"
	if variable, err := ParseVariables(test); err != nil {
		t.Errorf(`TestParseVar("%s") error: %s`, test, err.Error())
	} else {
		assert.True(t, variable.raw == "var",
			fmt.Sprintf(`TestParseVar("%s") value differ: %s`, variable.raw, "var"),
		)
		assert.True(t, len(variable.attributes) == 1,
			fmt.Sprintf(`TestParseVar("%s") attributes count differ - must be 1: %v`, variable.raw, variable.attributes),
		)
		assert.True(t, variable.attributes[0].raw == "attr1",
			fmt.Sprintf(`TestParseVar("%s") attribute value differ: %s`, variable.attributes[0].raw, "attr1"),
		)
		assert.True(t, variable.attributes[0].vartype == vartype_var,
			fmt.Sprintf(`TestParseVar("%s") vartype differ: %d`, variable.raw, variable.attributes[0].vartype),
		)
	}
}

// doc9
//
// parse $var1.${var2.attr1}.attr2
//
// => variable 'var1' and 2 attributes
//
// - 'var2' of type vartype_var with 1 attr
//
// - attr2 type attr without attribute
func TestParseVar9(t *testing.T) {
	initTest()

	test := "$var1.${var2.attr1}.attr2"
	if variable, err := ParseVariables(test); err != nil {
		t.Errorf(`TestParseVar("%s") error: %s`, test, err.Error())
	} else {
		assert.True(t, variable.raw == "var1",
			fmt.Sprintf(`TestParseVar("%s") value differ: %s`, variable.raw, "var1"),
		)
		assert.True(t, len(variable.attributes) == 2,
			fmt.Sprintf(`TestParseVar("%s") attributes count differ - must be 2: %v`, variable.raw, variable.attributes),
		)
		var sub_attr *Variable

		if len(variable.attributes) > 0 {
			sub_attr = variable.attributes[0]

			assert.True(t, sub_attr.raw == "var2",
				fmt.Sprintf(`TestParseVar("%s") attribute value differ: %s`, variable.attributes[0].raw, "var2"),
			)
			assert.True(t, sub_attr.vartype == vartype_var,
				fmt.Sprintf(`TestParseVar("%s") vartype differ: %d`, sub_attr.raw, sub_attr.vartype),
			)
			assert.True(t, len(sub_attr.attributes) == 1,
				fmt.Sprintf(`TestParseVar("%s") attributes count differ - must be 1: %v`, sub_attr.raw, sub_attr.attributes),
			)
			if len(sub_attr.attributes) > 1 {
				sub_attr = sub_attr.attributes[1]
				assert.True(t, sub_attr.raw == "attr1",
					fmt.Sprintf(`TestParseVar("%s") attribute value differ: %s`, sub_attr.raw, "attr1"),
				)
				assert.True(t, sub_attr.vartype == vartype_attribute,
					fmt.Sprintf(`TestParseVar("%s") vartype differ: %d`, sub_attr.raw, sub_attr.vartype),
				)
				assert.True(t, len(sub_attr.attributes) == 0,
					fmt.Sprintf(`TestParseVar("%s") attributes count differ - must be 0: %v`, sub_attr.raw, sub_attr.attributes),
				)
			}
		}

		if len(variable.attributes) > 1 {
			sub_attr = variable.attributes[1]
			assert.True(t, sub_attr.raw == "attr2",
				fmt.Sprintf(`TestParseVar("%s") attribute2 value differ: %s`, sub_attr.raw, "attr2"),
			)
			assert.True(t, sub_attr.vartype == vartype_attribute,
				fmt.Sprintf(`TestParseVar("%s") vartype differ: %d`, sub_attr.raw, sub_attr.vartype),
			)
			assert.True(t, len(sub_attr.attributes) == 0,
				fmt.Sprintf(`TestParseVar("%s") attributes count differ - must be 0: %v`, sub_attr.raw, sub_attr.attributes),
			)
		}
	}
}

//******************************************
//* valorize variables
//******************************************

// doc
//
//	symtab = {
//		"var": 1,
//	}
//
// check $var
//   - is defined
//   - is int
//   - equals 1
func TestValorizeValueInt(t *testing.T) {
	initTest()

	symtab["var"] = 1
	name, _ := NewField("$var", nil, nil)
	if r_value, err := ValorizeValue(symtab, name, logger, "test const int", false); err != nil {
		t.Errorf(`ValorizeValue("var") error: %s`, err.Error())
	} else {
		if value, ok := r_value.(int); ok {
			if value != 1 {
				t.Errorf(`ValorizeValue("$var") = %q want 1`, r_value)
			}
		} else {
			t.Errorf(`ValorizeValue("$var") not integer !`)
		}
	}
}

// doc
//
//	symtab = {
//		"var": "1",
//	}
//
// check $var
//   - is defined
//   - is string
//   - equals "1"
func TestValorizeValueConstString(t *testing.T) {
	initTest()

	symtab["var"] = "1"
	name, _ := NewField("$var", nil, nil)
	if r_value, err := ValorizeValue(symtab, name, logger, "test const string", false); err != nil {
		t.Errorf(`ValorizeValue("var") error: %s`, err.Error())
	} else {
		if value, ok := r_value.(string); ok {
			if value != "1" {
				t.Errorf(`ValorizeValue("$var") = %q want "1"`, r_value)
			}
		} else {
			t.Errorf(`ValorizeValue("$var") not string !`)
		}
	}
}

//**************************************************************************************
//**************************************************************************************

// doc
//
//	symtab = {
//		"slice": [
//			"value1",
//			"value2",
//		],
//	}
//
// check $slice
//   - is defined
//   - is []string
//   - len(slice) == 2
//   - slice[0] equals "value1"
func TestValorizeValueConstSlice(t *testing.T) {
	initTest()

	slice := make([]string, 2)
	slice[0] = "value1"
	slice[1] = "value2"
	symtab["slice"] = slice
	name, _ := NewField("$slice", nil, nil)
	if r_value, err := ValorizeValue(symtab, name, logger, "test const slice", false); err != nil {
		t.Errorf(`ValorizeValue("var") error: %s`, err.Error())
	} else {
		if value, ok := r_value.([]string); ok {
			if len(value) != 2 {
				t.Errorf(`ValorizeValue("$slice") = %q want []`, r_value)
			} else if value[0] != "value1" {
				t.Errorf(`ValorizeValue("$slice") = %q want "value1"`, value[0])
			}
		} else {
			t.Errorf(`ValorizeValue("$slice") not slice !`)
		}
	}
}

//**************************************************************************************
//**************************************************************************************

// doc
//
//	symtab = {
//		"mymap": {
//			"key1": "value1",
//			"key2": "value2",
//		},
//	}
//
// check $mymap
//   - is defined
//   - is map[string]string
//   - len(mymap) == 2
//   - mymap["key2"] equals "value2"
func TestValorizeValueConstMap(t *testing.T) {
	initTest()

	mymap := make(map[string]string)
	mymap["key1"] = "value1"
	mymap["key2"] = "value2"
	symtab["mymap"] = mymap
	name, _ := NewField("$mymap", nil, nil)
	if r_value, err := ValorizeValue(symtab, name, logger, "test constmap", false); err != nil {
		t.Errorf(`ValorizeValue("var") error: %s`, err.Error())
	} else {
		if value, ok := r_value.(map[string]string); ok {
			if len(value) != 2 {
				t.Errorf(`ValorizeValue("$mymap") = %q want map[string]string`, r_value)
			} else if value["key2"] != "value2" {
				t.Errorf(`ValorizeValue("$mymap") = %q want "value2"`, value["key2"])
			}
		} else {
			t.Errorf(`ValorizeValue("$mymap") not map !`)
		}
	}
}

//**************************************************************************************
//**************************************************************************************

// doc
//
//	symtab = {
//		"slice": [
//			"value1",
//			"value2",
//		],
//		"idx": 1
//	}
//
// check $slice[0]
//   - is defined
//   - is string
//   - equals "value1"
func TestValorizeValueConstSliceConstIndex(t *testing.T) {
	initTest()

	slice := make([]string, 2)
	slice[0] = "value1"
	slice[1] = "value2"
	symtab["slice"] = slice
	symtab["idx"] = 1

	var_name := `$slice[0]`
	name, _ := NewField(var_name, nil, nil)
	if r_value, err := ValorizeValue(symtab, name, logger, "test const slice", false); err != nil {
		assert.Nil(t, err, fmt.Sprintf(`ValorizeValue("%s") error: %s`, var_name, err.Error()))
	} else {
		if value, ok := r_value.(string); ok {
			assert.True(t, value == "value1",
				fmt.Sprintf(`ValorizeValue("%s") value differ: %s`, var_name, value),
			)
		} else {
			t.Errorf(`ValorizeValue("$slice") not slice !`)
		}
	}
}

//**************************************************************************************
//**************************************************************************************

// doc
//
//	symtab = {
//		"slice": [
//			"value1",
//			"value2",
//		],
//		"idx": 1
//	}
//
// check $slice[$idx]
//   - is defined
//   - is string
//   - equals "value2"
func TestValorizeValueConstSliceVarIndex(t *testing.T) {
	initTest()

	slice := make([]string, 2)
	slice[0] = "value1"
	slice[1] = "value2"
	symtab["slice"] = slice
	symtab["idx"] = 1

	var_name := `$slice[$idx]`
	name, _ := NewField(var_name, nil, nil)
	if r_value, err := ValorizeValue(symtab, name, logger, "test const slice", false); err != nil {
		assert.Nil(t, err, fmt.Sprintf(`ValorizeValue("%s") error: %s`, var_name, err.Error()))
	} else {
		if value, ok := r_value.(string); ok {
			assert.True(t, value == "value2",
				fmt.Sprintf(`ValorizeValue("%s") value differs`, var_name),
				value,
			)
		} else {
			t.Errorf(`ValorizeValue("$slice") not slice !`)
		}
	}
}

//**************************************************************************************
//**************************************************************************************

// doc
//
//	symtab = {
//		"mymap": {
//			"key1": "value1",
//			"key2": "value2",
//		},
//	}
//
// check $mymap["key1"]
//   - is defined
//   - is string
//   - is equals "value1"
func TestValorizeValueConstMapConstName(t *testing.T) {
	initTest()

	mymap := make(map[string]string)
	mymap["key1"] = "value1"
	mymap["key2"] = "value2"
	symtab["mymap"] = mymap
	var_name := `$mymap["key1"]`
	name, _ := NewField(var_name, nil, nil)
	if r_value, err := ValorizeValue(symtab, name, logger, "test constmap", false); err != nil {
		t.Errorf(`ValorizeValue("var") error: %s`, err.Error())
	} else {
		if value, ok := r_value.(string); ok {
			assert.True(t, value == "value1",
				fmt.Sprintf(`ValorizeValue("%s") value differs`, var_name),
				value,
			)
		} else {
			assert.True(t, ok, `ValorizeValue("$mymap") not map !`)
		}
	}
}

//**************************************************************************************
//**************************************************************************************

// doc
//
//	symtab = {
//		"mymap": {
//			"key1": "value1",
//			"key2": "value2",
//		},
//		"key": "key2"
//	}
//
// check $mymap[$key]
//   - is defined
//   - is string
//   - is equals "value2"
func TestValorizeValueConstMapVarName(t *testing.T) {
	initTest()

	mymap := make(map[string]string)
	mymap["key1"] = "value1"
	mymap["key2"] = "value2"
	symtab["mymap"] = mymap
	symtab["key"] = "key2"

	var_name := `$mymap[$key]`
	name, _ := NewField(var_name, nil, nil)

	if r_value, err := ValorizeValue(symtab, name, logger, "test constmap", false); err != nil {
		t.Errorf(`ValorizeValue("var") error: %s`, err.Error())
	} else {
		if value, ok := r_value.(string); ok {
			assert.True(t, value == "value2",
				fmt.Sprintf(`ValorizeValue("%s") value differs`, var_name),
				value,
			)
		} else {
			assert.True(t, ok, `ValorizeValue("$mymap") not map !`)
		}
	}
}

//**************************************************************************************
//**************************************************************************************

// doc
//
//	symtab = {
//		"mymap": {
//			"key1": "value1",
//			"key2": "value2",
//		},
//	}
//
// check go template func "exporterGet mymap "not_found"
//   - template is valid
//   - template return value string
func TestValorizeValueMapTemplateValidNotFound(t *testing.T) {
	initTest()

	mymap := make(map[string]string)
	mymap["key1"] = "value1"
	mymap["key2"] = "value2"
	symtab["mymap"] = mymap
	var_name := `{{ exporterGet .mymap "not_found" }}`

	name, err := NewField(var_name, nil, nil)
	if err != nil {
		t.Errorf(`ValorizeValue("%s") error: %s`, var_name, err.Error())
	}
	if r_value, err := ValorizeValue(symtab, name, logger, "test map template", false); err != nil {
		t.Errorf(`ValorizeValue("%s") error: %s`, var_name, err.Error())
	} else {
		if r_value != nil {
			t.Errorf(`ValorizeValue("%s") not nil %v!`, var_name, r_value)
		}
	}
}

//**************************************************************************************
//**************************************************************************************

// doc
//
//	symtab = {
//		"mymap": {
//			"key1": "value1",
//			"key2": "value2",
//		},
//	}
//
// check go template func "exporterGet mymap "key2"
//   - template is valid
//   - template return type is string
//   - template return value is "value2"
func TestValorizeValueMapTemplateValidKey(t *testing.T) {
	initTest()

	mymap := make(map[string]string)
	mymap["key1"] = "value1"
	mymap["key2"] = "value2"
	symtab["mymap"] = mymap
	var_name := `{{ exporterGet .mymap "key2" }}`

	name, err := NewField(var_name, nil, nil)
	if err != nil {
		t.Errorf(`ValorizeValue("%s") error: %s`, var_name, err.Error())
	}
	if r_value, err := ValorizeValue(symtab, name, logger, "test map template", false); err != nil {
		t.Errorf(`ValorizeValue("%s") error: %s`, var_name, err.Error())
	} else {
		if value, ok := r_value.(string); ok {
			if value != "value2" {
				t.Errorf(`ValorizeValue("%s") = %q want "value2"`, var_name, value)
			}
		} else {
			t.Errorf(`ValorizeValue("%s") not string !`, var_name)
		}
	}
}

//**************************************************************************************
//**************************************************************************************

// doc
//
//	symtab = {
//		"mymap": {
//			"key1": "value1",
//			"key2": "value2",
//		},
//	 "label": "$mymap"
//	}
//
// check :
//   - $mymap && $label variables creation
//   - $label var valorization
//   - $label is defined
//   - $label returned type is map[string][string]
func TestValorizeValueMap2Vars(t *testing.T) {
	initTest()
	var err error
	mymap := make(map[string]string)
	mymap["key1"] = "value1"
	mymap["key2"] = "value2"
	symtab["mymap"] = mymap
	var_name := `$label`
	symtab["label"], err = NewField("$mymap", nil, nil)
	if err != nil {
		t.Errorf(`ValorizeValue("%s") error: %s`, var_name, err.Error())
	}

	name, err := NewField(var_name, nil, nil)
	if err != nil {
		t.Errorf(`ValorizeValue("%s") error: %s`, var_name, err.Error())
	}
	if r_value, err := ValorizeValue(symtab, name, logger, "test map template", false); err != nil {
		t.Errorf(`ValorizeValue("%s") error: %s`, var_name, err.Error())
	} else {
		if map_val, ok := r_value.(map[string]string); !ok {
			t.Errorf(`ValorizeValue("%s") not map[string][string] ! %v`, var_name, r_value)
		} else {
			assert.True(t, map_val["key1"] == "value1", "invalid value for map[\"key1\"]", map_val)
		}
	}
}

//**************************************************************************************
//**************************************************************************************

// doc
//
//	symtab = {
//		"mymap": {
//			"key1": "value1",
//			"sub_map_1": {
//				"sub_key1": "sub_value1",
//				"sub_key2": "sub_value2",
//			}
//		}
//	}
//
// check $mymap.sub_map_1
//   - variables creation
//   - variables value is defined
//   - variables value has type map[string]string
func TestValorizeValueMap2LvlMap(t *testing.T) {
	var err error
	initTest()

	mymap := make(map[string]any)
	mymap["key1"] = "value1"
	sub_map := make(map[string]string)
	sub_map["sub_key1"] = "sub_value1"
	sub_map["sub_key2"] = "sub_value2"
	mymap["sub_map_1"] = sub_map
	symtab["mymap"] = mymap

	var_name := `$mymap.sub_map_1`
	name, err := NewField(var_name, nil, nil)
	if err != nil {
		t.Errorf(`ValorizeValue("%s") error: %s`, var_name, err.Error())
	}
	if r_value, err := ValorizeValue(symtab, name, logger, "test map template", false); err != nil {
		t.Errorf(`ValorizeValue("%s") error: %s`, var_name, err.Error())
	} else {
		if _, ok := r_value.(map[string]string); !ok {
			t.Errorf(`ValorizeValue("%s") not map[string][string] ! %v`, var_name, r_value)
		}
	}
}

//**************************************************************************************
//**************************************************************************************

// doc
//
//	symtab = {
//		"mymap": {
//			"key1": "value1",
//			"sub_list_1": [
//				"sub_value1",
//				"sub_value2",
//			],
//		}
//	}
//
// check $mymap.sub_list_1
//   - variables creation
//   - variables value is defined
//   - variables value has type []string
//   - variables len(value) == 2
//   - variables value[0] has type []string
func TestValorizeValueMap2LvlSlice(t *testing.T) {
	initTest()
	var err error
	mymap := make(map[string]any)
	mymap["key1"] = "value1"
	sub_slice := make([]string, 2)
	sub_slice[0] = "sub_value1"
	sub_slice[1] = "sub_value2"
	mymap["sub_list_1"] = sub_slice
	symtab["mymap"] = mymap

	var_name := `$mymap.sub_list_1`
	name, err := NewField(var_name, nil, nil)
	if err != nil {
		t.Errorf(`ValorizeValue("%s") error: %s`, var_name, err.Error())
	}
	if r_value, err := ValorizeValue(symtab, name, logger, "test map template", false); err != nil {
		t.Errorf(`ValorizeValue("%s") error: %s`, var_name, err.Error())
	} else {
		if s_val, ok := r_value.([]string); !ok {
			t.Errorf(`ValorizeValue("%s") not []string ! %v`, var_name, r_value)
		} else {
			assert.True(t, len(s_val) == 2, "invalid length for slice")
			if len(s_val) > 0 {
				assert.True(t, s_val[0] == "sub_value1", "invalid value for var", var_name, s_val[0])
			}
		}
	}
}

//**************************************************************************************
//**************************************************************************************

// doc
//
//	symtab = {
//		"mymap": {
//			"key1": "value1",
//			"sub_list_1": [
//				"sub_value1",
//				"sub_value2",
//			],
//		},
//		"list": "$mymap.sub_list_1"
//	}
//
// check $list
//   - variables creation
//   - variables value is defined
//   - variables value has type []string
//   - variables len(value) == 2
//   - variables value[0] has type []string
func TestValorizeValueMap2LvlSliceSubVar(t *testing.T) {
	initTest()
	var err error
	mymap := make(map[string]any)
	mymap["key1"] = "value1"
	sub_slice := make([]string, 2)
	sub_slice[0] = "sub_value1"
	sub_slice[1] = "sub_value2"
	mymap["sub_list_1"] = sub_slice
	symtab["mymap"] = mymap

	var_name := `$list`
	symtab["list"], err = NewField(`$mymap.sub_list_1`, nil, nil)
	if err != nil {
		t.Errorf(`ValorizeValue("%s") error: %s`, var_name, err.Error())
	}

	name, err := NewField(var_name, nil, nil)
	if err != nil {
		t.Errorf(`ValorizeValue("%s") error: %s`, var_name, err.Error())
	}
	if r_value, err := ValorizeValue(symtab, name, logger, "test map template", false); err != nil {
		t.Errorf(`ValorizeValue("%s") error: %s`, var_name, err.Error())
	} else {
		if _, ok := r_value.([]string); !ok {
			t.Errorf(`ValorizeValue("%s") not []string ! %v`, var_name, r_value)
		} else {
			if s_val, ok := r_value.([]string); !ok {
				t.Errorf(`ValorizeValue("%s") not []string ! %v`, var_name, r_value)
			} else {
				assert.True(t, len(s_val) == 2, "invalid length for slice")
				if len(s_val) > 0 {
					assert.True(t, s_val[0] == "sub_value1", "invalid value for var", var_name, s_val[0])
				}
			}
		}
	}
}

//**************************************************************************************
//**************************************************************************************

// doc
//
//	symtab = {
//		"mymap": {
//			"key1": "value1",
//			"sub_list_1": [
//				"sub_value1",
//				"sub_value2",
//			]
//		},
//		"list": "$mymap.sub_list_1"
//	}
//
// check $list[0]
//   - is defined
//   - is string
//   - equals  "sub_value1"
func TestValorizeValueMap2LvlSliceSubVarElement(t *testing.T) {
	initTest()

	var err error
	mymap := make(map[string]any)
	mymap["key1"] = "value1"
	sub_list := make([]string, 2)
	sub_list[0] = "sub_value1"
	sub_list[1] = "sub_value2"
	mymap["sub_list_1"] = sub_list
	symtab["mymap"] = mymap

	var_name := `$list[0]`
	symtab["list"], err = NewField(`$mymap.sub_list_1`, nil, nil)
	if err != nil {
		t.Errorf(`ValorizeValue("%s") error: %s`, var_name, err.Error())
	}

	name, err := NewField(var_name, nil, nil)
	if err != nil {
		t.Errorf(`ValorizeValue("%s") error: %s`, var_name, err.Error())
	}
	if r_value, err := ValorizeValue(symtab, name, logger, "test map template", false); err != nil {
		t.Errorf(`ValorizeValue("%s") error: %s`, var_name, err.Error())
	} else {
		if value, ok := r_value.(string); !ok {
			t.Errorf(`ValorizeValue("%s") not string ! %v`, var_name, r_value)
		} else if value != "sub_value1" {
			t.Errorf(`ValorizeValue("%s") not valid value ! %v`, var_name, r_value)
		}
	}
}

//**************************************************************************************
//**************************************************************************************

// doc
//
//	symtab = {
//		"mymap": {
//			"key1": "value1",
//			"sub_list_1": [
//				"sub_value1",
//				"sub_value2",
//			]
//		},
//		"mymap2": {
//			"name": "map_2",
//			"list": "$mymap.sub_list_1"
//		}
//	}
//
// check:
//   - mymap2["list"] is []string <==> mymap["sub_list_1"]
//   - check mymap2["list"][0] == "sub_value1"
func TestValorizeValueMap2LvlSliceSubVarSubElement(t *testing.T) {
	initTest()
	var err error
	mymap := make(map[string]any)
	mymap["key1"] = "value1"
	sub_list := make([]string, 2)
	sub_list[0] = "sub_value1"
	sub_list[1] = "sub_value2"
	mymap["sub_list_1"] = sub_list
	symtab["mymap"] = mymap

	mymap2 := make(map[string]any)
	mymap2["name"] = "map_2"
	// mymap2["list"] = "$mymap.sub_list_1"
	mymap2["list"], err = NewField(`$mymap.sub_list_1`, nil, nil)
	if err != nil {
		t.Errorf(`ValorizeValue("%s") error: %s`, `$mymap.sub_list_1`, err.Error())
	}
	symtab["mymap2"] = mymap2

	var_name := `$mymap2.list`
	// symtab["list"], err = NewField(`$mymap.sub_list_1`, nil)
	// if err != nil {
	// 	t.Errorf(`ValorizeValue("%s") error: %s`, var_name, err.Error())
	// }

	name, err := NewField(var_name, nil, nil)
	if err != nil {
		t.Errorf(`ValorizeValue("%s") error: %s`, var_name, err.Error())
	}
	if r_value, err := ValorizeValue(symtab, name, logger, "test map template", false); err != nil {
		t.Errorf(`ValorizeValue("%s") error: %s`, var_name, err.Error())
	} else {
		if v_list, ok := r_value.([]string); !ok {
			t.Errorf(`ValorizeValue("%s") not []string ! %v`, var_name, r_value)
		} else {
			assert.True(t, v_list[0] == "sub_value1")
		}
	}
}

//**************************************************************************************
//**************************************************************************************

// doc
//
//	symtab = {
//		"mymap": {
//			"key1": "value1",
//			"sub_map": {
//				"key1_1": "value1_1",
//				"key1_2": "value1_2",
//			}
//		},
//	 "var_name": "key1",
//	 "var_name2": "sub_map",
//	 "test": "$mymap.$var_name",
//	 "test2": "$mymap.$var_name2.key1_2",
//	}
//
// check:
//   - $test is string
//   - $test == "key1"
//   - $test2 is string
//   - $test2 == "value1_2"
func TestValorizeValueMap2LvlVarName2Var(t *testing.T) {
	initTest()
	var err error
	mymap := make(map[string]any)
	mymap["key1"] = "value1"
	sub_map := make(map[string]string, 2)
	sub_map["key1_1"] = "value1_1"
	sub_map["key1_2"] = "value1_2"
	mymap["sub_map"] = sub_map
	symtab["mymap"] = mymap

	symtab["var_name"] = "key1"
	// var_name := `$var_name`
	// symtab["list"], err = NewField(`$mymap.sub_list_1`, nil)
	// if err != nil {
	// 	t.Errorf(`ValorizeValue("%s") error: %s`, var_name, err.Error())
	// }

	test_name := `$mymap[$var_name]`
	name, err := NewField(test_name, nil, nil)
	if err != nil {
		t.Errorf(`ValorizeValue("%s") error: %s`, test_name, err.Error())
	} else {
		if r_value, err := ValorizeValue(symtab, name, logger, "test map template", false); err != nil {
			t.Errorf(`ValorizeValue("%s") error: %s`, test_name, err.Error())
		} else {
			if val, ok := r_value.(string); !ok {
				t.Errorf(`ValorizeValue("%s") not []string ! %v`, test_name, r_value)
			} else {
				assert.True(t, val == "value1")
			}
		}
	}

	symtab["var_name"] = "sub_map"
	test_name = `$mymap[$var_name].key1_2`
	name, err = NewField(test_name, nil, nil)
	if err != nil {
		t.Errorf(`ValorizeValue("%s") error: %s`, test_name, err.Error())
	} else {
		if r_value, err := ValorizeValue(symtab, name, logger, "test map template", false); err != nil {
			t.Errorf(`ValorizeValue("%s") error: %s`, test_name, err.Error())
		} else {
			if val, ok := r_value.(string); !ok {
				t.Errorf(`ValorizeValue("%s") not string ! %v`, test_name, r_value)
			} else {
				assert.True(t, val == "value1_2")
			}
		}
	}
}

//**************************************************************************************
//**************************************************************************************

// doc
//
//	symtab = {
//		 "mymap": {
//			"key1": "value1",
//			"sub_list": [
//				"sub_value1",
//				"sub_value2",
//			]
//		 },
//	  "test_var": [
//		    "sub_list",
//	   ],
//	  "var_name": "sub_list",
//	  "test": "$mymap[$var_name}][0]",
//	  "test2": "$mymap[$test_var[0]][1]",
//	}
//
// check:
//   - $test is string
//   - $test == "sub_value1"
//   - "$test2" is string
//   - $test2 == "sub_value2"
func TestValorizeValueMap2LvlVarName2VarWithIndex(t *testing.T) {
	initTest()
	var err error
	mymap := make(map[string]any)
	mymap["key1"] = "value1"
	sub_list := make([]string, 2)
	sub_list[0] = "sub_value1"
	sub_list[1] = "sub_value2"
	mymap["sub_list"] = sub_list
	symtab["mymap"] = mymap
	test_var_list := make([]string, 2)
	test_var_list[0] = "sub_list"
	symtab["test_var"] = test_var_list

	symtab["var_name"] = "sub_list"

	test_name := `$mymap[$var_name][0]`
	name, err := NewField(test_name, nil, nil)
	if err != nil {
		t.Errorf(`ValorizeValue("%s") error: %s`, test_name, err.Error())
	} else {
		if r_value, err := ValorizeValue(symtab, name, logger, "test map template", false); err != nil {
			t.Errorf(`ValorizeValue("%s") error: %s`, test_name, err.Error())
		} else {
			if val, ok := r_value.(string); !ok {
				t.Errorf(`ValorizeValue("%s") not string ! %v`, test_name, r_value)
			} else {
				assert.True(t, val == "sub_value1")
			}
		}
	}

	test_name = `$mymap[$test_var[0]][1]`
	name, err = NewField(test_name, nil, nil)
	if err != nil {
		t.Errorf(`ValorizeValue("%s") error: %s`, test_name, err.Error())
	} else {
		if r_value, err := ValorizeValue(symtab, name, logger, "test map template", false); err != nil {
			t.Errorf(`ValorizeValue("%s") error: %s`, test_name, err.Error())
		} else {
			if val, ok := r_value.(string); !ok {
				t.Errorf(`ValorizeValue("%s") not string ! %v`, test_name, r_value)
			} else {
				assert.True(t, val == "sub_value2")
			}
		}
	}
}
