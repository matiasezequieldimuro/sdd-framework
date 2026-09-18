package domain

type Config struct {
	SchemaVersion string         `json:"schema_version" yaml:"schema_version"`
	Defaults      ConfigDefaults `json:"defaults" yaml:"defaults"`
}

type ConfigDefaults struct {
	Workflow string `json:"workflow" yaml:"workflow"`
}
