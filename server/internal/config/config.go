package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type WhatsAppMode string

const (
	WhatsAppOff  WhatsAppMode = "off"
	WhatsAppTest WhatsAppMode = "test"
	WhatsAppLive WhatsAppMode = "live"
)

type Config struct {
	Addr              string
	DataDir           string
	WhatsAppMode      WhatsAppMode
	WhatsAppAllowlist []string
	AdminToken        string
	PublicURL         string
	Timezone          string
}

/**
 * NOTE: Settings come from LOUVADEIRA_* environment variables. WhatsApp defaults to test mode so a fresh
 * install can never message anyone outside the allowlist.
 */
func Load() (Config, error) {
	cfg := Config{
		Addr:              envOr("LOUVADEIRA_ADDR", ":8090"),
		DataDir:           envOr("LOUVADEIRA_DATA_DIR", "data"),
		WhatsAppMode:      WhatsAppMode(envOr("LOUVADEIRA_WHATSAPP_MODE", string(WhatsAppTest))),
		WhatsAppAllowlist: splitList(os.Getenv("LOUVADEIRA_WHATSAPP_ALLOWLIST")),
		AdminToken:        os.Getenv("LOUVADEIRA_ADMIN_TOKEN"),
		PublicURL:         strings.TrimRight(envOr("LOUVADEIRA_PUBLIC_URL", "http://localhost:8090"), "/"),
		Timezone:          envOr("LOUVADEIRA_TIMEZONE", "America/Sao_Paulo"),
	}
	switch cfg.WhatsAppMode {
	case WhatsAppOff, WhatsAppTest, WhatsAppLive:
	default:
		return Config{}, fmt.Errorf("LOUVADEIRA_WHATSAPP_MODE inválido: %q (use off, test ou live)", cfg.WhatsAppMode)
	}
	return cfg, nil
}

func (c Config) AppDBPath() string {
	return filepath.Join(c.DataDir, "app.db")
}

func (c Config) WhatsAppDBPath() string {
	return filepath.Join(c.DataDir, "whatsapp.db")
}

func envOr(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		return value
	}
	return fallback
}

func splitList(raw string) []string {
	var items []string
	for _, item := range strings.Split(raw, ",") {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			items = append(items, trimmed)
		}
	}
	return items
}
