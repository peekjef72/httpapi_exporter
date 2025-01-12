package main

import (
	"log/slog"
	"testing"
)

var (
	symtab map[string]any
	logger *slog.Logger
)

func initTest() {
	symtab = make(map[string]any)
	logger = &slog.Logger{}

}
func TestValorizeValueInt(t *testing.T) {
	initTest()

	symtab["var"] = 1
	name, _ := NewField("$var", nil)
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
func TestValorizeValueConstString(t *testing.T) {
	initTest()

	symtab["var"] = "1"
	name, _ := NewField("$var", nil)
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

func TestValorizeValueConstSlice(t *testing.T) {
	initTest()

	slice := make([]string, 2)
	slice[0] = "value1"
	slice[1] = "value2"
	symtab["slice"] = slice
	name, _ := NewField("$slice", nil)
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

func TestValorizeValueConstMap(t *testing.T) {
	initTest()

	mymap := make(map[string]string)
	mymap["key1"] = "value1"
	mymap["key2"] = "value2"
	symtab["mymap"] = mymap
	name, _ := NewField("$mymap", nil)
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

func TestValorizeValueMapTemplateInValid(t *testing.T) {
	initTest()

	mymap := make(map[string]string)
	mymap["key1"] = "value1"
	mymap["key2"] = "value2"
	symtab["mymap"] = mymap
	var_name := `{{ exporterGet .mymap "not_found" }}`

	name, err := NewField(var_name, nil)
	if err != nil {
		t.Errorf(`ValorizeValue("%s") error: %s`, var_name, err.Error())
	}
	if r_value, err := ValorizeValue(symtab, name, logger, "test maptemplate", false); err != nil {
		t.Errorf(`ValorizeValue("%s") error: %s`, var_name, err.Error())
	} else {
		if r_value != nil {
			t.Errorf(`ValorizeValue("%s") not nil %v!`, var_name, r_value)
		}
	}
}

func TestValorizeValueMapTemplateValid(t *testing.T) {
	initTest()

	mymap := make(map[string]string)
	mymap["key1"] = "value1"
	mymap["key2"] = "value2"
	symtab["mymap"] = mymap
	var_name := `{{ exporterGet .mymap "key2" }}`

	name, err := NewField(var_name, nil)
	if err != nil {
		t.Errorf(`ValorizeValue("%s") error: %s`, var_name, err.Error())
	}
	if r_value, err := ValorizeValue(symtab, name, logger, "test maptemplate", false); err != nil {
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

func TestValorizeValueMap2Vars(t *testing.T) {
	initTest()
	var err error
	mymap := make(map[string]string)
	mymap["key1"] = "value1"
	mymap["key2"] = "value2"
	symtab["mymap"] = mymap
	var_name := `$label`
	symtab["label"], err = NewField("$mymap", nil)
	if err != nil {
		t.Errorf(`ValorizeValue("%s") error: %s`, var_name, err.Error())
	}

	name, err := NewField(var_name, nil)
	if err != nil {
		t.Errorf(`ValorizeValue("%s") error: %s`, var_name, err.Error())
	}
	if r_value, err := ValorizeValue(symtab, name, logger, "test maptemplate", false); err != nil {
		t.Errorf(`ValorizeValue("%s") error: %s`, var_name, err.Error())
	} else {
		if _, ok := r_value.(map[string]string); !ok {
			t.Errorf(`ValorizeValue("%s") not map[string][string] ! %v`, var_name, r_value)
		}
	}
}

func TestValorizeValueMap2LvlMap(t *testing.T) {
	initTest()
	var err error
	mymap := make(map[string]any)
	mymap["key1"] = "value1"
	sub_map := make(map[string]string)
	sub_map["sub_key1"] = "sub_value1"
	sub_map["sub_key2"] = "sub_value2"
	mymap["sub_map_1"] = sub_map
	symtab["mymap"] = mymap

	var_name := `$mymap.sub_map_1`
	name, err := NewField(var_name, nil)
	if err != nil {
		t.Errorf(`ValorizeValue("%s") error: %s`, var_name, err.Error())
	}
	if r_value, err := ValorizeValue(symtab, name, logger, "test maptemplate", false); err != nil {
		t.Errorf(`ValorizeValue("%s") error: %s`, var_name, err.Error())
	} else {
		if _, ok := r_value.(map[string]string); !ok {
			t.Errorf(`ValorizeValue("%s") not map[string][string] ! %v`, var_name, r_value)
		}
	}
}

func TestValorizeValueMap2LvlSlice(t *testing.T) {
	initTest()
	var err error
	mymap := make(map[string]any)
	mymap["key1"] = "value1"
	sub_map := make([]string, 2)
	sub_map[0] = "sub_value1"
	sub_map[1] = "sub_value2"
	mymap["sub_list_1"] = sub_map
	symtab["mymap"] = mymap

	var_name := `$mymap.sub_list_1`
	name, err := NewField(var_name, nil)
	if err != nil {
		t.Errorf(`ValorizeValue("%s") error: %s`, var_name, err.Error())
	}
	if r_value, err := ValorizeValue(symtab, name, logger, "test maptemplate", false); err != nil {
		t.Errorf(`ValorizeValue("%s") error: %s`, var_name, err.Error())
	} else {
		if _, ok := r_value.([]string); !ok {
			t.Errorf(`ValorizeValue("%s") not []string ! %v`, var_name, r_value)
		}
	}
}

func TestValorizeValueMap2LvlSliceSubVar(t *testing.T) {
	initTest()
	var err error
	mymap := make(map[string]any)
	mymap["key1"] = "value1"
	sub_map := make([]string, 2)
	sub_map[0] = "sub_value1"
	sub_map[1] = "sub_value2"
	mymap["sub_list_1"] = sub_map
	symtab["mymap"] = mymap

	var_name := `$list`
	symtab["list"], err = NewField(`$mymap.sub_list_1`, nil)
	if err != nil {
		t.Errorf(`ValorizeValue("%s") error: %s`, var_name, err.Error())
	}

	name, err := NewField(var_name, nil)
	if err != nil {
		t.Errorf(`ValorizeValue("%s") error: %s`, var_name, err.Error())
	}
	if r_value, err := ValorizeValue(symtab, name, logger, "test maptemplate", false); err != nil {
		t.Errorf(`ValorizeValue("%s") error: %s`, var_name, err.Error())
	} else {
		if _, ok := r_value.([]string); !ok {
			t.Errorf(`ValorizeValue("%s") not []string ! %v`, var_name, r_value)
		}
	}
}

func TestValorizeValueMap2LvlSliceSubVarElmt(t *testing.T) {
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
	symtab["list"], err = NewField(`$mymap.sub_list_1`, nil)
	if err != nil {
		t.Errorf(`ValorizeValue("%s") error: %s`, var_name, err.Error())
	}

	name, err := NewField(var_name, nil)
	if err != nil {
		t.Errorf(`ValorizeValue("%s") error: %s`, var_name, err.Error())
	}
	if r_value, err := ValorizeValue(symtab, name, logger, "test maptemplate", false); err != nil {
		t.Errorf(`ValorizeValue("%s") error: %s`, var_name, err.Error())
	} else {
		if value, ok := r_value.(string); !ok {
			t.Errorf(`ValorizeValue("%s") not []string ! %v`, var_name, r_value)
		} else if value != "sub_value1" {
			t.Errorf(`ValorizeValue("%s") not string ! %v`, var_name, r_value)
		}
	}
}

func TestValorizeValueMap2LvlSliceSubVarSubElmt(t *testing.T) {
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
	mymap2["list"] = "$mymap.sub_list_1"
	mymap2["list"], err = NewField(`$mymap.sub_list_1`, nil)
	if err != nil {
		t.Errorf(`ValorizeValue("%s") error: %s`, `$mymap.sub_list_1`, err.Error())
	}
	symtab["mymap2"] = mymap2

	var_name := `$mymap2.list`
	// symtab["list"], err = NewField(`$mymap.sub_list_1`, nil)
	// if err != nil {
	// 	t.Errorf(`ValorizeValue("%s") error: %s`, var_name, err.Error())
	// }

	name, err := NewField(var_name, nil)
	if err != nil {
		t.Errorf(`ValorizeValue("%s") error: %s`, var_name, err.Error())
	}
	if r_value, err := ValorizeValue(symtab, name, logger, "test maptemplate", false); err != nil {
		t.Errorf(`ValorizeValue("%s") error: %s`, var_name, err.Error())
	} else {
		if _, ok := r_value.([]string); !ok {
			t.Errorf(`ValorizeValue("%s") not []string ! %v`, var_name, r_value)
		}
	}
}
