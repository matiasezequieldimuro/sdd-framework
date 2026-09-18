package infra

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/santhosh-tekuri/jsonschema/v5"
	"gopkg.in/yaml.v3"

	"sdd-cli/internal/domain"
)

type SchemaValidator struct{}

type SchemaViolation struct {
	InstancePath string
	SchemaPath   string
	Message      string
}

func NewSchemaValidator() *SchemaValidator {
	return &SchemaValidator{}
}

func (validator *SchemaValidator) ValidateYAML(baseDir, schemaFile string, data []byte) error {
	violations, err := validator.ValidateYAMLAll(baseDir, schemaFile, data)
	if err != nil {
		return err
	}
	if len(violations) > 0 {
		return fmt.Errorf("%w: %s", domain.ErrSchemaValidation, violations[0].Message)
	}
	return nil
}

func (validator *SchemaValidator) ValidateYAMLAll(
	baseDir, schemaFile string,
	data []byte,
) ([]SchemaViolation, error) {
	var value interface{}
	if err := yaml.Unmarshal(data, &value); err != nil {
		return nil, fmt.Errorf("%w: parse YAML: %v", domain.ErrSchemaValidation, err)
	}
	return validator.ValidateValueAll(baseDir, schemaFile, value)
}

func (validator *SchemaValidator) ValidateValue(baseDir, schemaFile string, value interface{}) error {
	violations, err := validator.ValidateValueAll(baseDir, schemaFile, value)
	if err != nil {
		return err
	}
	if len(violations) > 0 {
		return fmt.Errorf("%w: %s", domain.ErrSchemaValidation, violations[0].Message)
	}
	return nil
}

func (validator *SchemaValidator) Compile(baseDir, schemaFile string) error {
	_, err := validator.compile(baseDir, schemaFile)
	return err
}

func (validator *SchemaValidator) ValidateValueAll(
	baseDir, schemaFile string,
	value interface{},
) ([]SchemaViolation, error) {
	schema, err := validator.compile(baseDir, schemaFile)
	if err != nil {
		return nil, err
	}

	jsonData, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("%w: encode value: %v", domain.ErrSchemaValidation, err)
	}
	var jsonValue interface{}
	decoder := json.NewDecoder(bytes.NewReader(jsonData))
	decoder.UseNumber()
	if err := decoder.Decode(&jsonValue); err != nil {
		return nil, fmt.Errorf("%w: decode value: %v", domain.ErrSchemaValidation, err)
	}
	if err := schema.Validate(jsonValue); err != nil {
		var validationError *jsonschema.ValidationError
		if !errors.As(err, &validationError) {
			return nil, fmt.Errorf("%w: %v", domain.ErrSchemaValidation, err)
		}
		violations := flattenSchemaViolations(validationError)
		sort.Slice(violations, func(i, j int) bool {
			if violations[i].InstancePath != violations[j].InstancePath {
				return violations[i].InstancePath < violations[j].InstancePath
			}
			if violations[i].SchemaPath != violations[j].SchemaPath {
				return violations[i].SchemaPath < violations[j].SchemaPath
			}
			return violations[i].Message < violations[j].Message
		})
		return violations, nil
	}

	return nil, nil
}

func (validator *SchemaValidator) compile(baseDir, schemaFile string) (*jsonschema.Schema, error) {
	schemaPath, err := containedPath(filepath.Join(baseDir, ".sdd"), "schemas", schemaFile)
	if err != nil {
		return nil, err
	}
	schemaData, err := os.ReadFile(schemaPath)
	if err != nil {
		return nil, fmt.Errorf("%w: read %s: %v", domain.ErrSchemaValidation, schemaFile, err)
	}

	compiler := jsonschema.NewCompiler()
	compiler.AssertFormat = true
	if err := compiler.AddResource(schemaFile, bytes.NewReader(schemaData)); err != nil {
		return nil, fmt.Errorf("%w: load %s: %v", domain.ErrSchemaValidation, schemaFile, err)
	}
	schema, err := compiler.Compile(schemaFile)
	if err != nil {
		return nil, fmt.Errorf("%w: compile %s: %v", domain.ErrSchemaValidation, schemaFile, err)
	}
	return schema, nil
}

func flattenSchemaViolations(validationError *jsonschema.ValidationError) []SchemaViolation {
	if len(validationError.Causes) == 0 {
		return []SchemaViolation{{
			InstancePath: validationError.InstanceLocation,
			SchemaPath:   validationError.KeywordLocation,
			Message:      validationError.Message,
		}}
	}
	var violations []SchemaViolation
	for _, cause := range validationError.Causes {
		violations = append(violations, flattenSchemaViolations(cause)...)
	}
	return violations
}
