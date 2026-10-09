package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Workers != 4 {
		t.Errorf("expected 4 workers, got %d", cfg.Workers)
	}
	if cfg.Format != "ass" {
		t.Errorf("expected ass format, got %s", cfg.Format)
	}
}

func TestLoad_CustomFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.yaml")
	data := `
api:
  model: custom-model
workers: 8
`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Workers != 8 {
		t.Errorf("expected 8 workers, got %d", cfg.Workers)
	}
	if cfg.API.Model != "custom-model" {
		t.Errorf("expected custom-model, got %s", cfg.API.Model)
	}
	// defaults preserved
	if cfg.Format != "ass" {
		t.Errorf("expected default ass, got %s", cfg.Format)
	}
}

func TestLoad_MissingFile(t *testing.T) {
	cfg, err := Load("does_not_exist.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Workers != 4 {
		t.Errorf("expected default 4 workers")
	}
}

