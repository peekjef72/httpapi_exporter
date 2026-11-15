package main

import (
	"fmt"
	"log/slog"
	"os"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestQueryActionConfig_UnmarshalYAML(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		wantErr  bool
		validate func(*testing.T, *QueryActionConfig)
	}{
		{
			name:    "valid minimal config",
			yaml:    "url: \"/api/test\"",
			wantErr: false,
			validate: func(t *testing.T, qc *QueryActionConfig) {
				if qc.Method != "GET" {
					t.Errorf("expected default method GET, got %s", qc.Method)
				}
				if qc.VarName != "_root" {
					t.Errorf("expected default var_name _root, got %s", qc.VarName)
				}
				if qc.Parser != "json" {
					t.Errorf("expected default parser json, got %s", qc.Parser)
				}
				if qc.FollowRedirection != true {
					t.Errorf("expected default follow_redirection true")
				}
			},
		},
		{
			name: "valid full config",
			yaml: `
url: "/api/test"
method: "POST"
data: "some data"
var_name: "my_result"
parser: "xml"
ok_status: [200, 201]
timeout: 10
`,
			wantErr: false,
			validate: func(t *testing.T, qc *QueryActionConfig) {
				if qc.Method != "POST" {
					t.Errorf("expected POST, got %s", qc.Method)
				}
				if qc.VarName != "my_result" {
					t.Errorf("expected my_result, got %s", qc.VarName)
				}
				if qc.Parser != "xml" {
					t.Errorf("expected xml, got %s", qc.Parser)
				}
			},
		},
		{
			name:    "invalid parser",
			yaml:    "url: \"/test\"\nparser: \"invalid\"",
			wantErr: true,
		},
		{
			name:    "missing url",
			yaml:    "method: \"GET\"",
			wantErr: true,
		},
		{
			name:    "invalid template in url",
			yaml:    "url: \"{{ .invalid_syntax\"",
			wantErr: true,
		},
		{
			name:    "overflow fields",
			yaml:    "url: \"/test\"\nunknown_field: \"value\"",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var qc QueryActionConfig
			err := yaml.Unmarshal([]byte(tt.yaml), &qc)
			if (err != nil) != tt.wantErr {
				t.Errorf("UnmarshalYAML() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && tt.validate != nil {
				tt.validate(t, &qc)
			}
		})
	}
}

func TestBuildStatus(t *testing.T) {
	tests := []struct {
		name     string
		raw      any
		expected []int
	}{
		{
			name:     "single int",
			raw:      200,
			expected: []int{200},
		},
		{
			name:     "slice of ints",
			raw:      []any{200, 201, 204},
			expected: []int{200, 201, 204},
		},
		{
			name:     "slice of strings",
			raw:      []any{"200", "201"},
			expected: []int{200, 201},
		},
		{
			name:     "mixed slice",
			raw:      []any{200, "201", "invalid"},
			expected: []int{200, 201, 0},
		},
		{
			name:     "nil",
			raw:      nil,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := buildStatus(tt.raw)
			if len(res) != len(tt.expected) {
				t.Errorf("expected len %d, got %d", len(tt.expected), len(res))
				return
			}
			for i := range res {
				if res[i] != tt.expected[i] {
					t.Errorf("at index %d: expected %d, got %d", i, tt.expected[i], res[i])
				}
			}
		})
	}
}

func TestQueryAction_CustomAction(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	symtab := map[string]any{
		"url_var":    "/api/v1/resource",
		"method_var": "PUT",
	}

	// Mock internal method
	mockMethod := func(params *CallClientExecuteParams, st map[string]any) error {
		if params.Url != "/api/v1/resource" {
			return fmt.Errorf("wrong URL")
		}
		if params.Method != "PUT" {
			return fmt.Errorf("wrong method")
		}
		return nil
	}
	symtab["__method"] = mockMethod

	action := &QueryAction{
		Name: &Field{raw: "test_query"},
		Query: &QueryActionConfig{
			Query:   "{{ .url_var }}",
			Method:  "{{ .method_var }}",
			VarName: "result",
		},
	}
	// Initialize required fields manually
	action.Query.query, _ = NewField(action.Query.Query, nil, nil)
	action.Query.method, _ = NewField(action.Query.Method, nil, nil)
	action.Query.var_name, _ = NewField(action.Query.VarName, nil, nil)
	action.Name, _ = NewField("test_query", nil, nil)

	err := action.CustomAction(nil, symtab, logger)
	if err != nil {
		t.Errorf("CustomAction() failed: %v", err)
	}
}
