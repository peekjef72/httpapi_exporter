package main

import (
	"log/slog"
	"os"
	"reflect"
	"testing"
)

func TestParseVariables(t *testing.T) {
	tests := []struct {
		name     string
		expr     string
		expected string
		wantErr  bool
	}{
		{
			name:     "simple variable",
			expr:     "$var",
			expected: "$var",
			wantErr:  false,
		},
		{
			name:     "variable with attribute",
			expr:     "$var.attr",
			expected: "$var.attr",
			wantErr:  false,
		},
		{
			name:     "variable with nested attribute",
			expr:     "$var.attr1.attr2",
			expected: "$var.attr1.attr2",
			wantErr:  false,
		},
		{
			name:     "simple slice",
			expr:     "$var[0]",
			expected: "$var[0]",
			wantErr:  false,
		},
		{
			name:     "variable with dynamic index",
			expr:     "$var[$index]",
			expected: "$var[$index]",
			wantErr:  false,
		},
		{
			name:     "variable with dynamic attribute",
			expr:     "$var[$var2]",
			expected: "$var[$var2]",
			wantErr:  false,
		},
		{
			name:     "complex nested",
			expr:     "$var.attr[$index].subattr",
			expected: "$var.attr[$index].subattr",
			wantErr:  false,
		},
		{
			name:    "invalid start",
			expr:    "var",
			wantErr: true,
		},
		{
			name:    "unterminated string",
			expr:    "$var[\"unfinished",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := ParseVariables(tt.expr)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseVariables(%q) error = %v, wantErr %v", tt.expr, err, tt.wantErr)
				return
			}
			if !tt.wantErr && v.String() != tt.expected {
				t.Errorf("ParseVariables(%q) = %q, want %q", tt.expr, v.String(), tt.expected)
			}
		})
	}
}

func TestVariable_GetVar(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	symtab := map[string]any{
		"user": "alice",
		"settings": map[string]any{
			"theme": "dark",
			"notifications": map[string]any{
				"email": true,
			},
		},
		"scores": []any{10, 20, 30},
		"matrix": []any{
			map[string]any{"id": 1, "val": "a"},
			map[string]any{"id": 2, "val": "b"},
		},
		"index": 1,
	}

	tests := []struct {
		name     string
		expr     string
		expected any
		wantErr  bool
	}{
		{
			name:     "direct variable",
			expr:     "$user",
			expected: "alice",
			wantErr:  false,
		},
		{
			name:     "simple attribute",
			expr:     "$settings.theme",
			expected: "dark",
			wantErr:  false,
		},
		{
			name:     "nested attribute",
			expr:     "$settings.notifications.email",
			expected: true,
			wantErr:  false,
		},
		{
			name:     "slice index",
			expr:     "$scores[1]",
			expected: 20,
			wantErr:  false,
		},
		{
			name:     "dynamic slice index",
			expr:     "$scores[$index]",
			expected: 20,
			wantErr:  false,
		},
		{
			name:     "nested map in slice",
			expr:     "$matrix[0].val",
			expected: "a",
			wantErr:  false,
		},
		{
			name:     "dynamic index in slice",
			expr:     "$matrix[$index].val",
			expected: "b",
			wantErr:  false,
		},
		{
			name:    "non existent variable",
			expr:    "$unknown",
			wantErr: true,
		},
		{
			name:    "non existent attribute",
			expr:    "$settings.missing",
			wantErr: true,
		},
		{
			name:    "out of bounds index",
			expr:    "$scores[10]",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := ParseVariables(tt.expr)
			if err != nil {
				t.Fatalf("ParseVariables failed: %v", err)
			}

			res, err := v.GetVar(symtab, nil, logger)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetVar(%q) error = %v, wantErr %v", tt.expr, err, tt.wantErr)
				return
			}
			if !tt.wantErr && res != tt.expected {
				t.Errorf("GetVar(%q) = %v, want %v", tt.expr, res, tt.expected)
			}
		})
	}
}

func TestVariable_SetVar(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	tests := []struct {
		name       string
		expr       string
		value      any
		symtab     map[string]any
		wantSymtab map[string]any
		wantErr    bool
	}{
		{
			name:   "set simple variable - new",
			expr:   "$user",
			value:  "bob",
			symtab: map[string]any{},
			wantSymtab: map[string]any{
				"user": "bob",
			},
			wantErr: false,
		},
		{
			name:  "set simple variable - existing",
			expr:  "$user",
			value: "charlie",
			symtab: map[string]any{
				"user": "bob",
			},
			wantSymtab: map[string]any{
				"user": "charlie",
			},
			wantErr: false,
		},
		{
			name:  "set map attribute - existing",
			expr:  "$settings.theme",
			value: "light",
			symtab: map[string]any{
				"settings": map[string]any{
					"theme": "dark",
				},
			},
			wantSymtab: map[string]any{
				"settings": map[string]any{
					"theme": "light",
				},
			},
			wantErr: false,
		},
		{
			name:  "set map attribute - new key",
			expr:  "$settings.font",
			value: "Arial",
			symtab: map[string]any{
				"settings": map[string]any{
					"theme": "dark",
				},
			},
			wantSymtab: map[string]any{
				"settings": map[string]any{
					"theme": "dark",
					"font":  "Arial",
				},
			},
			wantErr: false,
		},
		{
			name:    "set map attribute - new root and new key",
			expr:    "$new_map.key",
			value:   "val",
			symtab:  map[string]any{},
			wantErr: true, // Should fail because root must exist for attributes
		},
		{
			name:  "set map attribute - existing root empty map",
			expr:  "$empty_map.key",
			value: "val",
			symtab: map[string]any{
				"empty_map": map[string]any{},
			},
			wantSymtab: map[string]any{
				"empty_map": map[string]any{
					"key": "val",
				},
			},
			wantErr: false,
		},
		{
			name:  "set slice index - existing",
			expr:  "$scores[1]",
			value: 25,
			symtab: map[string]any{
				"scores": []any{10, 20, 30},
			},
			wantSymtab: map[string]any{
				"scores": []any{10, 25, 30},
			},
			wantErr: false,
		},
		{
			name:  "set slice index - out of bounds",
			expr:  "$scores[10]",
			value: 100,
			symtab: map[string]any{
				"scores": []any{10, 20, 30},
			},
			wantErr: true,
		},
		{
			name:  "set non-existent map path",
			expr:  "$settings.missing.key",
			value: "val",
			symtab: map[string]any{
				"settings": map[string]any{
					"theme": "dark",
				},
			},
			wantErr: true,
		},
		{
			name:  "set on non-container type",
			expr:  "$user.name",
			value: "bob",
			symtab: map[string]any{
				"user": "alice",
			},
			wantErr: true,
		},
		{
			name:  "set dynamic attribute",
			expr:  "$settings[$key]",
			value: "dynamic_val",
			symtab: map[string]any{
				"key": "theme",
				"settings": map[string]any{
					"theme": "dark",
				},
			},
			wantSymtab: map[string]any{
				"key": "theme",
				"settings": map[string]any{
					"theme": "dynamic_val",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := ParseVariables(tt.expr)
			if err != nil {
				t.Fatalf("ParseVariables failed: %v", err)
			}

			err = v.SetVar(tt.symtab, tt.value, logger)
			if (err != nil) != tt.wantErr {
				t.Errorf("SetVar(%q) error = %v, wantErr %v", tt.expr, err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				for k, expectedVal := range tt.wantSymtab {
					actualVal := tt.symtab[k]
					if !reflect.DeepEqual(actualVal, expectedVal) {
						t.Errorf("SetVar(%q) symtab[%q] = %v, want %v", tt.expr, k, actualVal, expectedVal)
					}
				}
			}
		})
	}
}
