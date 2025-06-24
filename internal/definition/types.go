package definition

import "strings"

type Definition struct {
	Version    int               `yaml:"version"`
	Params     map[string]string `yaml:"params"`
	Operations []Operation       `yaml:"operations"`
}

type Operation struct {
	ID              string                   `yaml:"id,omitempty"`
	Description     string                   `yaml:"description,omitempty"`
	Type            string                   `yaml:"type,omitempty"`
	SQL             string                   `yaml:"sql"`
	Expected        []map[string]interface{} `yaml:"expected,omitempty"`
	ExpectedChanges map[string]int           `yaml:"expected_changes,omitempty"`
}

type Report struct {
	ID          string      `json:"id"`
	Description string      `json:"description"`
	Type        string      `json:"type"`
	Result      interface{} `json:"result"`
	Pass        bool        `json:"pass"`
	Message     string      `json:"message"`
}

const (
	TypeSelect = "select"
	TypeInsert = "insert"
	TypeUpdate = "update"
	TypeDelete = "delete"
)

var AllowedTypes = []string{TypeSelect, TypeInsert, TypeUpdate, TypeDelete}

// DetectSQLType SQLクエリから操作タイプを自動判定
func DetectSQLType(sql string) string {
	normalized := strings.TrimSpace(sql)
	normalized = strings.ToUpper(normalized)

	if strings.HasPrefix(normalized, "SELECT") {
		return TypeSelect
	}
	if strings.HasPrefix(normalized, "INSERT") {
		return TypeInsert
	}
	if strings.HasPrefix(normalized, "UPDATE") {
		return TypeUpdate
	}
	if strings.HasPrefix(normalized, "DELETE") {
		return TypeDelete
	}

	return ""
}
