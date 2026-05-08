package config

import (
	"fmt"
	"os"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL        string
	JWTSecret          string
	JWTExpiry          time.Duration
	RefreshExpiry      time.Duration
	ResetTokenExpiry   time.Duration
	FrontendURL        string
	StorageDriver      string
	LocalStoragePath   string
	S3Bucket           string
	S3Region           string
	Port               string
	AllowedOrigins     string
	SMTPHost           string
	SMTPPort           string
	SMTPUsername       string
	SMTPPassword       string
	SMTPFrom           string
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{
		DatabaseURL:      os.Getenv("DATABASE_URL"),
		JWTSecret:        os.Getenv("JWT_SECRET"),
		FrontendURL:      getEnvDefault("FRONTEND_URL", "http://localhost:3000"),
		StorageDriver:    getEnvDefault("STORAGE_DRIVER", "local"),
		LocalStoragePath: getEnvDefault("LOCAL_STORAGE_PATH", "./uploads"),
		S3Bucket:         os.Getenv("S3_BUCKET"),
		S3Region:         os.Getenv("S3_REGION"),
		Port:             getEnvDefault("PORT", "8080"),
		AllowedOrigins:   getEnvDefault("ALLOWED_ORIGINS", "*"),
		SMTPHost:         os.Getenv("SMTP_HOST"),
		SMTPPort:         getEnvDefault("SMTP_PORT", "587"),
		SMTPUsername:     os.Getenv("SMTP_USERNAME"),
		SMTPPassword:     os.Getenv("SMTP_PASSWORD"),
		SMTPFrom:         os.Getenv("SMTP_FROM"),
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}

	jwtExpiry, err := parseDurationDefault("JWT_EXPIRY", time.Hour)
	if err != nil {
		return nil, fmt.Errorf("invalid JWT_EXPIRY: %w", err)
	}
	cfg.JWTExpiry = jwtExpiry

	refreshExpiry, err := parseDurationDefault("REFRESH_EXPIRY", 168*time.Hour)
	if err != nil {
		return nil, fmt.Errorf("invalid REFRESH_EXPIRY: %w", err)
	}
	cfg.RefreshExpiry = refreshExpiry

	resetTokenExpiry, err := parseDurationDefault("RESET_TOKEN_EXPIRY", time.Hour)
	if err != nil {
		return nil, fmt.Errorf("invalid RESET_TOKEN_EXPIRY: %w", err)
	}
	cfg.ResetTokenExpiry = resetTokenExpiry

	if cfg.StorageDriver != "local" && cfg.StorageDriver != "s3" {
		return nil, fmt.Errorf("STORAGE_DRIVER must be 'local' or 's3', got %q", cfg.StorageDriver)
	}
	if cfg.StorageDriver == "s3" {
		if cfg.S3Bucket == "" {
			return nil, fmt.Errorf("S3_BUCKET is required when STORAGE_DRIVER=s3")
		}
		if cfg.S3Region == "" {
			return nil, fmt.Errorf("S3_REGION is required when STORAGE_DRIVER=s3")
		}
	}

	return cfg, nil
}

func getEnvDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func parseDurationDefault(key string, def time.Duration) (time.Duration, error) {
	v := os.Getenv(key)
	if v == "" {
		return def, nil
	}
	return time.ParseDuration(v)
}
