package definition

import (
	"bytes"
	"fmt"
	"os"
	"text/template"

	"gopkg.in/yaml.v3"
)

func LoadDefinitions(configPaths []string) (*Definition, error) {
	if len(configPaths) == 0 {
		return nil, fmt.Errorf("no configuration files specified")
	}
	
	if len(configPaths) == 1 {
		return LoadDefinition(configPaths[0])
	}
	
	// Load and merge multiple configuration files
	var mergedDef *Definition
	for i, configPath := range configPaths {
		def, err := LoadDefinition(configPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load config file %s: %w", configPath, err)
		}
		
		if i == 0 {
			mergedDef = def
		} else {
			if err := mergeDefinitions(mergedDef, def); err != nil {
				return nil, fmt.Errorf("failed to merge config file %s: %w", configPath, err)
			}
		}
	}
	
	return mergedDef, nil
}

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

// mergeDefinitions merges two definitions together
func mergeDefinitions(base, additional *Definition) error {
	// Version validation - all files should have the same version
	if base.Version != additional.Version {
		return fmt.Errorf("version mismatch: base has version %d, additional has version %d", base.Version, additional.Version)
	}
	
	// Merge parameters - additional params override base params
	if base.Params == nil {
		base.Params = make(map[string]interface{})
	}
	for key, value := range additional.Params {
		base.Params[key] = value
	}
	
	// Check for duplicate operation IDs
	existingIDs := make(map[string]bool)
	for _, op := range base.Operations {
		if op.ID != "" {
			existingIDs[op.ID] = true
		}
	}
	
	// Append operations from additional definition
	for i, op := range additional.Operations {
		if op.ID != "" && existingIDs[op.ID] {
			return fmt.Errorf("duplicate operation ID: %s", op.ID)
		}
		
		// If ID is empty, it will be auto-generated during validation
		// But we need to track it to avoid duplicates
		if op.ID == "" {
			// Auto-generate ID for checking duplicates
			autoID := fmt.Sprintf("operation_%d", len(base.Operations)+i)
			if existingIDs[autoID] {
				return fmt.Errorf("auto-generated operation ID conflict: %s", autoID)
			}
		}
		
		base.Operations = append(base.Operations, op)
		if op.ID != "" {
			existingIDs[op.ID] = true
		}
	}
	
	return nil
}
