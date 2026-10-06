package domain_test

import (
	"testing"

	"gopkg.in/yaml.v3"

	"sdd-cli/internal/domain"
)

// TestTokenUsageSetting_TolerantUnmarshal verifies D-d: only YAML boolean true
// activates the audit; all other values (false, string "optional", absent)
// yield inactive.
func TestTokenUsageSetting_TolerantUnmarshal(t *testing.T) {
	cases := []struct {
		name      string
		yaml      string
		wantActive bool
	}{
		{
			name:      "boolean true -> active",
			yaml:      "observability:\n  token_usage: true\n",
			wantActive: true,
		},
		{
			name:      "boolean false -> inactive",
			yaml:      "observability:\n  token_usage: false\n",
			wantActive: false,
		},
		{
			name:      "string optional -> inactive (legacy compat)",
			yaml:      "observability:\n  token_usage: optional\n",
			wantActive: false,
		},
		{
			name:      "absent field -> inactive",
			yaml:      "observability: {}\n",
			wantActive: false,
		},
		{
			name:      "absent observability -> inactive",
			yaml:      "{}\n",
			wantActive: false,
		},
		{
			name:      "string 'false' -> inactive",
			yaml:      "observability:\n  token_usage: 'false'\n",
			wantActive: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var obs domain.ConfigObservability
			// Wrap in a minimal struct so we can unmarshal via the Observability key.
			wrapper := struct {
				Observability domain.ConfigObservability `yaml:"observability"`
			}{}
			if err := yaml.Unmarshal([]byte(tc.yaml), &wrapper); err != nil {
				t.Fatalf("yaml.Unmarshal() error = %v", err)
			}
			obs = wrapper.Observability
			_ = obs // ensure it's used
			cfg := &domain.Config{Observability: obs}
			if got := cfg.TokenAuditActive(); got != tc.wantActive {
				t.Errorf("TokenAuditActive() = %v, want %v", got, tc.wantActive)
			}
		})
	}
}
