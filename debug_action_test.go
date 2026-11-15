package main

import (
	"log/slog"
	"os"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestDebugActionConfig_UnmarshalYAML(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr bool
	}{
		{
			name:    "valid config",
			yaml:    "msg: \"hello debug\"\nlevel: \"info\"",
			wantErr: false,
		},
		{
			name:    "valid config default level",
			yaml:    "msg: \"hello debug\"",
			wantErr: false,
		},
		{
			name:    "missing msg",
			yaml:    "level: \"info\"",
			wantErr: true,
		},
		{
			name:    "invalid template",
			yaml:    "msg: \"{{ .invalid_syntax\"",
			wantErr: true,
		},
		{
			name:    "overflow fields",
			yaml:    "msg: \"hello\"\nunknown_field: \"value\"",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var dc DebugActionConfig
			err := yaml.Unmarshal([]byte(tt.yaml), &dc)
			if (err != nil) != tt.wantErr {
				t.Errorf("UnmarshalYAML() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDebugAction_CustomAction(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	symtab := map[string]any{
		"test_var": "some value",
	}

	action := &DebugAction{
		Name: &Field{raw: "test_debug"},
		Debug: &DebugActionConfig{
			MsgVal: "Variable is {{ .test_var }}",
			Level:  "info",
		},
	}
	// Manual initialization of fields since we aren't using UnmarshalYAML here
	action.Debug.msg, _ = NewField(action.Debug.MsgVal, nil, nil)
	action.Debug.level, _ = NewField(action.Debug.Level, nil, nil)
	action.Name, _ = NewField("test_debug", nil, nil)

	err := action.CustomAction(nil, symtab, logger)
	if err != nil {
		t.Errorf("CustomAction() failed: %v", err)
	}
}

func TestDebugAction_GettersSetters(t *testing.T) {
	a := &DebugAction{}

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
