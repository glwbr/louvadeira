package config_test

import (
	"reflect"
	"testing"

	"louvadeira/internal/config"
)

func TestLoadDefaultsToWhatsAppTestMode(t *testing.T) {
	t.Setenv("LOUVADEIRA_WHATSAPP_MODE", "")
	t.Setenv("LOUVADEIRA_WHATSAPP_ALLOWLIST", " 1@g.us , ,5511988887777 ")
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.WhatsAppMode != config.WhatsAppTest {
		t.Fatalf("mode = %q, want test", cfg.WhatsAppMode)
	}
	if want := []string{"1@g.us", "5511988887777"}; !reflect.DeepEqual(cfg.WhatsAppAllowlist, want) {
		t.Fatalf("allowlist = %v, want %v", cfg.WhatsAppAllowlist, want)
	}
}

func TestLoadRejectsUnknownWhatsAppMode(t *testing.T) {
	t.Setenv("LOUVADEIRA_WHATSAPP_MODE", "producao")
	if _, err := config.Load(); err == nil {
		t.Fatal("expected an error for an unknown mode")
	}
}
