package main

import (
	"log/slog"
	"os"
	"reflect"
	"testing"

	"github.com/peekjef72/httpapi_exporter/goja_modules"
	"github.com/peekjef72/httpapi_exporter/template"
)

func TestSetFactAction_CustomAction(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	registry, _ := goja_modules.InitJSRegistry(logger, template.Js_func_map())
	// jsField, _ := NewField("js: input.toUpperCase()", nil, registry)
	// code := `
	// 	js:
	// 		config.timeout=60;
	// 		config.retry = 3;
	// 		0
	// `
	// jsField2, _ := NewField(code, nil, registry)

	// dynField, _ := NewField("$dyn_key", nil, nil)

	tests := []struct {
		name           string
		initialSymtab  map[string]any
		setFact        [][]any // Simulates the internal a.setFact
		expectedSymtab map[string]any
		wantErr        bool
	}{
		{
			name: "set simple fact",
			initialSymtab: map[string]any{
				"existing": "old",
			},
			setFact: [][]any{
				{&Field{raw: "new_var"}, "new_value"},
			},
			expectedSymtab: map[string]any{
				"existing": "old",
				"new_var":  "new_value",
			},
			wantErr: false,
		},
		{
			name: "update existing fact",
			initialSymtab: map[string]any{
				"var1": "old_value",
			},
			setFact: [][]any{
				{&Field{raw: "var1"}, "updated_value"},
			},
			expectedSymtab: map[string]any{
				"var1": "updated_value",
			},
			wantErr: false,
		},
		{
			name: "remove fact (value is nil)",
			initialSymtab: map[string]any{
				"to_delete": "",
				"keep":      "stay",
			},
			setFact: [][]any{
				{&Field{raw: "to_delete"}, nil},
			},
			expectedSymtab: map[string]any{
				"keep": "stay",
			},
			wantErr: false,
		},
		{
			name:          "forbidden variable name",
			initialSymtab: map[string]any{},
			setFact: [][]any{
				{&Field{raw: "__forbidden"}, "value"},
			},
			expectedSymtab: nil,
			wantErr:        true,
		},
		{
			name: "discard result (key is '_')",
			initialSymtab: map[string]any{
				"keep": "stay",
			},
			setFact: [][]any{
				{&Field{raw: "_"}, "discard_me"},
			},
			expectedSymtab: map[string]any{
				"keep": "stay",
			},
			wantErr: false,
		},
		{
			name: "set nested map value (static path)",
			initialSymtab: map[string]any{
				"config": map[string]any{
					"timeout": 30,
				},
			},
			setFact: [][]any{
				{&Field{raw: "$config.timeout", vartype: field_var}, 60},
				{&Field{raw: "$config.retry", vartype: field_var}, 3},
			},
			expectedSymtab: map[string]any{
				"config": map[string]any{
					"timeout": 60,
					"retry":   3,
				},
			},
			wantErr: false,
		},
		{
			name: "js formulation via Field",
			initialSymtab: map[string]any{
				"input": "hello",
			},
			setFact: [][]any{
				// {&Field{raw: "input"}, jsField},
				{&Field{raw: "input"}, &Field{raw: "js: input.toUpperCase()", vartype: field_js}},
			},
			expectedSymtab: map[string]any{
				"input": "HELLO",
			},
			wantErr: false,
		},
		{
			name: "set nested map value (via js)",
			initialSymtab: map[string]any{
				"config": map[string]any{
					"timeout": 30,
				},
			},
			setFact: [][]any{
				// {&Field{raw: "_"}, jsField2},
				{&Field{raw: "_"}, &Field{raw: `
					js:
						config.timeout=60;
						config.retry = 3;
						0
				`, vartype: field_js,
				}},
			},
			expectedSymtab: map[string]any{
				"config": map[string]any{
					"timeout": int64(60),
					"retry":   int64(3),
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			symtab := make(map[string]any)
			symtab["__collector_id"] = "TestSetFactionAction"
			symtab["__name__"] = "CustomAction"
			for k, v := range tt.initialSymtab {
				symtab[k] = v
			}

			for i, pair := range tt.setFact {
				for j, raw_act := range pair {
					if action, ok := raw_act.(*Field); ok {
						if action.vartype != field_raw {
							action, _ = NewField(action.raw, nil, registry)
							tt.setFact[i][j] = action
						}
					}
				}
			}
			action := &SetFactAction{
				Name:    &Field{raw: "test_set_fact"},
				setFact: tt.setFact,
			}

			err := action.CustomAction(nil, symtab, logger)
			if (err != nil) != tt.wantErr {
				t.Errorf("CustomAction() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			delete(symtab, "__collector_id")
			delete(symtab, "__name__")
			if !tt.wantErr && tt.expectedSymtab != nil {
				for k, v := range tt.expectedSymtab {
					actual := symtab[k]
					if reflect.DeepEqual(actual, v) == false {
						t.Errorf("expected symtab[%s] = %v, got %v", k, v, actual)
					}
				}
				if len(symtab) != len(tt.expectedSymtab) {
					t.Errorf("symtab size mismatch: expected %d, got %d", len(tt.expectedSymtab), len(symtab))
				}
			}
		})
	}
}

func TestSetFactAction_GettersSetters(t *testing.T) {
	a := &SetFactAction{}

	nameField := &Field{raw: "test"}
	a.SetNameField(nameField)
	if a.GetNameField() != nameField {
		t.Error("SetNameField/GetNameField failed")
	}

	with := []any{"a", "b"}
	a.SetWith(with)
	if len(a.GetWith()) != 2 {
		t.Error("SetWith/GetWith failed")
	}

	when := []*Field{{raw: "cond"}}
	a.SetWhen(when)
	if len(a.GetWhen()) != 1 {
		t.Error("SetWhen/GetWhen failed")
	}

	a.SetLoopVar("i")
	if a.GetLoopVar() != "i" {
		t.Error("SetLoopVar/GetLoopVar failed")
	}
}
