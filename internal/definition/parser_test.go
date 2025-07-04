package definition

import (
	"os"
	"strings"
	"testing"
)

func TestMergeDefinitions(t *testing.T) {
	tests := []struct {
		name      string
		base      *Definition
		additional *Definition
		wantError bool
		errorMsg  string
	}{
		{
			name: "merge with explicit IDs - no duplicates",
			base: &Definition{
				Version: 1,
				Operations: []Operation{
					{ID: "op1", SQL: "SELECT 1"},
				},
			},
			additional: &Definition{
				Version: 1,
				Operations: []Operation{
					{ID: "op2", SQL: "SELECT 2"},
				},
			},
			wantError: false,
		},
		{
			name: "merge with explicit duplicate IDs",
			base: &Definition{
				Version: 1,
				Operations: []Operation{
					{ID: "op1", SQL: "SELECT 1"},
				},
			},
			additional: &Definition{
				Version: 1,
				Operations: []Operation{
					{ID: "op1", SQL: "SELECT 2"},
				},
			},
			wantError: true,
			errorMsg:  "duplicate operation ID: op1",
		},
		{
			name: "merge with no IDs (auto-generated)",
			base: &Definition{
				Version: 1,
				Operations: []Operation{
					{SQL: "SELECT 1"},
				},
			},
			additional: &Definition{
				Version: 1,
				Operations: []Operation{
					{SQL: "SELECT 2"},
				},
			},
			wantError: false,
		},
		{
			name: "merge with mixed IDs - no duplicates",
			base: &Definition{
				Version: 1,
				Operations: []Operation{
					{ID: "explicit1", SQL: "SELECT 1"},
					{SQL: "SELECT 2"}, // will be auto-generated
				},
			},
			additional: &Definition{
				Version: 1,
				Operations: []Operation{
					{ID: "explicit2", SQL: "SELECT 3"},
					{SQL: "SELECT 4"}, // will be auto-generated
				},
			},
			wantError: false,
		},
		{
			name: "version mismatch",
			base: &Definition{
				Version: 1,
				Operations: []Operation{
					{SQL: "SELECT 1"},
				},
			},
			additional: &Definition{
				Version: 2,
				Operations: []Operation{
					{SQL: "SELECT 2"},
				},
			},
			wantError: true,
			errorMsg:  "version mismatch",
		},
		{
			name: "merge parameters",
			base: &Definition{
				Version: 1,
				Params: map[string]string{
					"param1": "value1",
					"param2": "value2",
				},
				Operations: []Operation{
					{SQL: "SELECT 1"},
				},
			},
			additional: &Definition{
				Version: 1,
				Params: map[string]string{
					"param2": "override",
					"param3": "value3",
				},
				Operations: []Operation{
					{SQL: "SELECT 2"},
				},
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := mergeDefinitions(tt.base, tt.additional)
			
			if tt.wantError {
				if err == nil {
					t.Errorf("expected error but got none")
					return
				}
				if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error message to contain '%s', got '%s'", tt.errorMsg, err.Error())
				}
				return
			}
			
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			
			// Verify merge results
			if tt.name == "merge parameters" {
				expectedParams := map[string]string{
					"param1": "value1",
					"param2": "override", // should be overridden
					"param3": "value3",
				}
				
				for key, expectedValue := range expectedParams {
					if actualValue, exists := tt.base.Params[key]; !exists || actualValue != expectedValue {
						t.Errorf("expected param %s=%s, got %s=%s", key, expectedValue, key, actualValue)
					}
				}
			}
		})
	}
}

func TestLoadDefinitionsMultipleFiles(t *testing.T) {
	// Create temporary test files
	tests := []struct {
		name      string
		files     []string
		contents  []string
		wantError bool
		errorMsg  string
	}{
		{
			name: "two files with no ID duplicates",
			files: []string{"test1.yaml", "test2.yaml"},
			contents: []string{
				`version: 1
operations:
  - id: op1
    sql: "SELECT 1"
    type: select
    expected:
      - count: 1
`,
				`version: 1
operations:
  - id: op2
    sql: "SELECT 2"  
    type: select
    expected:
      - count: 1
`,
			},
			wantError: false,
		},
		{
			name: "two files with duplicate operation_0",
			files: []string{"test1.yaml", "test2.yaml"},
			contents: []string{
				`version: 1
operations:
  - id: operation_0
    sql: "SELECT 1"
    type: select
    expected:
      - count: 1
`,
				`version: 1
operations:
  - id: operation_0
    sql: "SELECT 2"
    type: select  
    expected:
      - count: 1
`,
			},
			wantError: true,
			errorMsg:  "duplicate operation ID: operation_0",
		},
		{
			name: "two files with auto-generated IDs",
			files: []string{"test1.yaml", "test2.yaml"},
			contents: []string{
				`version: 1
operations:
  - sql: "SELECT 1"
    type: select
    expected:
      - count: 1
`,
				`version: 1
operations:
  - sql: "SELECT 2"
    type: select
    expected:
      - count: 1
`,
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary files
			var tempFiles []string
			for i, content := range tt.contents {
				tempFile := t.TempDir() + "/" + tt.files[i]
				if err := writeTestFile(tempFile, content); err != nil {
					t.Fatalf("failed to create test file: %v", err)
				}
				tempFiles = append(tempFiles, tempFile)
			}

			// Test LoadDefinitions
			def, err := LoadDefinitions(tempFiles)
			
			if tt.wantError {
				if err == nil {
					t.Errorf("expected error but got none")
					return
				}
				if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error message to contain '%s', got '%s'", tt.errorMsg, err.Error())
				}
				return
			}
			
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			
			if def == nil {
				t.Error("expected definition but got nil")
				return
			}
			
			// For auto-generated ID test, verify IDs are unique
			if tt.name == "two files with auto-generated IDs" {
				if len(def.Operations) != 2 {
					t.Errorf("expected 2 operations, got %d", len(def.Operations))
				}
				
				// Check that all operations have unique IDs after validation
				ids := make(map[string]bool)
				for _, op := range def.Operations {
					if op.ID == "" {
						t.Error("operation should have ID after validation")
					}
					if ids[op.ID] {
						t.Errorf("duplicate ID found: %s", op.ID)
					}
					ids[op.ID] = true
				}
			}
		})
	}
}

// Helper function to write test files
func writeTestFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0644)
}