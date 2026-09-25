package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigProviderAPIBase(t *testing.T) {
	cases := []struct {
		name     string
		base     string
		override string
		want     string
	}{
		{"root URL", "https://llm.example.com", "", "https://llm.example.com/v1"},
		{"root URL with slash", "https://llm.example.com/", "", "https://llm.example.com/v1"},
		{"versioned URL", "https://llm.example.com/v1", "", "https://llm.example.com/v1"},
		{"custom path", "https://llm.example.com/openai/v1/", "", "https://llm.example.com/openai/v1"},
		{"environment override", "https://llm.example.com/v1", "https://other.example.com", "https://other.example.com/v1"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("LOCALAGENT_API_BASE", tc.override)
			path := filepath.Join(t.TempDir(), "config.json")
			if err := os.WriteFile(path, []byte(`{"provider":{"api_base":"`+tc.base+`"}}`), 0600); err != nil {
				t.Fatal(err)
			}
			cfg, err := LoadConfig(path)
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Provider.APIBase != tc.want {
				t.Fatalf("APIBase = %q, want %q", cfg.Provider.APIBase, tc.want)
			}
		})
	}
}
