package definition

import (
	"bytes"
	"fmt"
	"os"
	"text/template"

	"gopkg.in/yaml.v3"
)

func LoadDefinition(configPath string) (*Definition, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %s %w", configPath, err)
	}

	var def Definition
	if err := yaml.Unmarshal(data, &def); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	if err := def.Validate(); err != nil {
		return nil, err
	}

	if err := def.ProcessTemplates(); err != nil {
		return nil, err
	}

	return &def, nil
}

func (d *Definition) Validate() error {
	if d.Version != 1 && d.Version != 0 {
		return fmt.Errorf("unsupported version: %d", d.Version)
	}

	for i, op := range d.Operations {
		if op.SQL == "" {
			return fmt.Errorf("operation[%d]: sql is required", i)
		}

		// IDが未指定の場合はインデックスを使用
		opID := op.ID
		if opID == "" {
			opID = fmt.Sprintf("operation_%d", i)
			d.Operations[i].ID = opID
		}

		// Typeが未指定の場合はSQLから自動判定
		opType := op.Type
		if opType == "" {
			opType = DetectSQLType(op.SQL)
			if opType == "" {
				return fmt.Errorf("operation[%s]: unable to detect SQL type from query", opID)
			}
			// 自動判定されたタイプを設定
			d.Operations[i].Type = opType
		}

		if !contains(AllowedTypes, opType) {
			return fmt.Errorf("operation[%s]: unsupported type: %s (allowed: %v)", opID, opType, AllowedTypes)
		}

		if opType == TypeSelect && len(op.Expected) == 0 {
			return fmt.Errorf("operation[%s]: expected is required for SELECT", opID)
		}
		if opType != TypeSelect && len(op.ExpectedChanges) == 0 {
			return fmt.Errorf("operation[%s]: expected_changes is required for DML", opID)
		}
	}

	return nil
}

func (d *Definition) ProcessTemplates() error {
	for i, op := range d.Operations {
		opID := op.ID
		if opID == "" {
			opID = fmt.Sprintf("operation_%d", i)
		}

		tmpl, err := template.New(opID).Parse(op.SQL)
		if err != nil {
			return fmt.Errorf("operation[%s]: failed to parse SQL template: %w", opID, err)
		}

		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, map[string]interface{}{
			"params": d.Params,
		}); err != nil {
			return fmt.Errorf("operation[%s]: failed to execute SQL template: %w", opID, err)
		}

		d.Operations[i].SQL = buf.String()
	}

	return nil
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
