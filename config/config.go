package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port        string
	DBPath      string
	HLSDir      string
	HLSTime     int
	HLSListSize int
}

func Load() *Config {
	return &Config{
		Port:        env("PORT", "8080"),
		DBPath:      env("DB_PATH", "onvif.db"),
		HLSDir:      env("HLS_DIR", "hls"),
		HLSTime:     envInt("HLS_TIME", 2),
		HLSListSize: envInt("HLS_LIST_SIZE", 3),
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
