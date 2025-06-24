package test

import (
	"testing"

	"github.com/pyama86/opsql/internal/definition"
)

func TestDetectSQLType(t *testing.T) {
	tests := []struct {
		name     string
		sql      string
		expected string
	}{
		{
			name:     "SELECT query",
			sql:      "SELECT * FROM users",
			expected: "select",
		},
		{
			name:     "INSERT query",
			sql:      "INSERT INTO users (name) VALUES ('test')",
			expected: "insert",
		},
		{
			name:     "UPDATE query",
			sql:      "UPDATE users SET name = 'test'",
			expected: "update",
		},
		{
			name:     "DELETE query",
			sql:      "DELETE FROM users WHERE id = 1",
			expected: "delete",
		},
		{
			name:     "SELECT with whitespace",
			sql:      "  \n  SELECT id FROM users  ",
			expected: "select",
		},
		{
			name:     "lowercase select",
			sql:      "select * from users",
			expected: "select",
		},
		{
			name:     "multiline UPDATE",
			sql:      "UPDATE users\nSET status = 'inactive'\nWHERE id = 1",
			expected: "update",
		},
		{
			name:     "unknown query",
			sql:      "TRUNCATE TABLE users",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := definition.DetectSQLType(tt.sql)
			if result != tt.expected {
				t.Errorf("DetectSQLType() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestLoadDefinitionWithAutoDetection(t *testing.T) {
	def, err := definition.LoadDefinition("../examples/simple.yaml")
	if err != nil {
		t.Fatalf("LoadDefinition() error = %v", err)
	}

	if len(def.Operations) != 4 {
		t.Errorf("Expected 4 operations, got %d", len(def.Operations))
	}

	expectedTypes := []string{"select", "update", "insert", "delete"}
	for i, op := range def.Operations {
		if op.Type != expectedTypes[i] {
			t.Errorf("Operation[%d] type = %v, want %v", i, op.Type, expectedTypes[i])
		}
	}
}
