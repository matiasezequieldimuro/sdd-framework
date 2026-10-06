package domain

import "gopkg.in/yaml.v3"

// Config is the root structure of the `.sdd/config.yaml` file.
type Config struct {
	SchemaVersion string         `json:"schema_version" yaml:"schema_version"`
	Defaults      ConfigDefaults `json:"defaults" yaml:"defaults"`
	// Observability holds optional cross-cutting observability settings.
	Observability ConfigObservability `json:"observability" yaml:"observability"`
}

// ConfigDefaults holds project-level defaults (workflow, language, …).
type ConfigDefaults struct {
	Workflow string `json:"workflow" yaml:"workflow"`
}

// ConfigObservability groups the observability-related config fields.
type ConfigObservability struct {
	// TokenUsage controls the token-usage audit feature per work item.
	TokenUsage TokenUsageSetting `json:"token_usage" yaml:"token_usage"`
}

// TokenUsageSetting is a boolean-like type whose YAML unmarshal is tolerant
// (D-d): only the literal boolean true activates the audit; false, the string
// "optional", an absent field, or any other value yields inactive. This
// ensures existing configs with `token_usage: optional` continue to parse
// without error (RC-5 / no-break guarantee).
type TokenUsageSetting bool

// UnmarshalYAML implements yaml.Unmarshaler with tolerant semantics:
//   - YAML boolean true  → active (true)
//   - anything else      → inactive (false)
func (t *TokenUsageSetting) UnmarshalYAML(value *yaml.Node) error {
	// Accept boolean nodes only; everything else maps to false.
	if value.Kind == yaml.ScalarNode && value.Tag == "!!bool" {
		var b bool
		if err := value.Decode(&b); err != nil {
			// If decoding a bool tag fails, default to inactive.
			*t = false
			return nil
		}
		*t = TokenUsageSetting(b)
		return nil
	}
	// String literals ("optional", "false", …) and any other type → inactive.
	*t = false
	return nil
}

// TokenAuditActive reports whether the token-usage audit is enabled. It
// returns true only when the config explicitly sets token_usage to true.
func (c *Config) TokenAuditActive() bool {
	return bool(c.Observability.TokenUsage)
}
