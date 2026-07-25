package queue

import (
	"testing"
	"time"
)

func TestArgsKind(t *testing.T) {
	if (IngestArgs{}).Kind() != "ingest" {
		t.Errorf("IngestArgs.Kind() = %q", (IngestArgs{}).Kind())
	}
	if (SweepArgs{}).Kind() != "sweep_stale" {
		t.Errorf("SweepArgs.Kind() = %q", (SweepArgs{}).Kind())
	}
}

func TestConfigWithDefaults(t *testing.T) {
	got := Config{}.withDefaults()
	if got.IngestInterval != 6*time.Hour {
		t.Errorf("IngestInterval = %s, want 6h", got.IngestInterval)
	}
	if got.SweepInterval != time.Hour {
		t.Errorf("SweepInterval = %s, want 1h", got.SweepInterval)
	}
	if got.StaleAfter != 14*24*time.Hour {
		t.Errorf("StaleAfter = %s, want 336h", got.StaleAfter)
	}

	custom := Config{IngestInterval: time.Minute, SweepInterval: 2 * time.Minute, StaleAfter: time.Hour}
	if custom.withDefaults() != custom {
		t.Errorf("explicit values must be preserved, got %+v", custom.withDefaults())
	}
}
