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
		return nil, fmt.Errorf("failed to read config file: %w", err)
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
	if d.Version != 1 {
		return fmt.Errorf("unsupported version: %d", d.Version)
	}

	for i, op := range d.Operations {
		if op.ID == "" {
			return fmt.Errorf("operation[%d]: id is required", i)
		}
		if op.Type == "" {
			return fmt.Errorf("operation[%s]: type is required", op.ID)
		}
		if !contains(AllowedTypes, op.Type) {
			return fmt.Errorf("operation[%s]: unsupported type: %s (allowed: %v)", op.ID, op.Type, AllowedTypes)
		}
		if op.SQL == "" {
			return fmt.Errorf("operation[%s]: sql is required", op.ID)
		}

		if op.Type == TypeSelect && len(op.Expected) == 0 {
			return fmt.Errorf("operation[%s]: expected is required for SELECT", op.ID)
		}
		if op.Type != TypeSelect && len(op.ExpectedChanges) == 0 {
			return fmt.Errorf("operation[%s]: expected_changes is required for DML", op.ID)
		}
	}

	return nil
}

func (d *Definition) ProcessTemplates() error {
	for i, op := range d.Operations {
		tmpl, err := template.New(op.ID).Parse(op.SQL)
		if err != nil {
			return fmt.Errorf("operation[%s]: failed to parse SQL template: %w", op.ID, err)
		}

		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, map[string]interface{}{
			"params": d.Params,
		}); err != nil {
			return fmt.Errorf("operation[%s]: failed to execute SQL template: %w", op.ID, err)
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
