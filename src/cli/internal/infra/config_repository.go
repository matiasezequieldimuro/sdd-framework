package infra

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"sdd-cli/internal/domain"
	"sdd-cli/internal/ports"
)

type FSConfigRepository struct{}

func NewFSConfigRepository() ports.ConfigRepository {
	return &FSConfigRepository{}
}

func (repository *FSConfigRepository) GetConfig(baseDir string) (*domain.Config, error) {
	configPath, err := containedPath(filepath.Join(baseDir, ".sdd"), "config.yaml")
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config.yaml: %w", err)
	}

	var config domain.Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config.yaml: %w", err)
	}
	if err := domain.ValidateContractSchemaVersion("config.yaml", config.SchemaVersion); err != nil {
		return nil, err
	}
	if err := domain.ValidateIdentifier("default workflow", config.Defaults.Workflow); err != nil {
		return nil, err
	}

	return &config, nil
}
