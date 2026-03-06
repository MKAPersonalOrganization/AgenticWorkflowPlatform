package main

import (
	"os"
	"school-with-ai/foundation/logger"
	"strings"
	"testing"
)

func testLogger() *logger.Logger {
	return logger.NewTextWithEvents(os.Stderr, logger.LevelInfo, "TEST", nil, logger.Events{})
}

func TestProductionConfigGuardDevelopBypass(t *testing.T) {
	// In develop mode, insecure defaults should be accepted.
	err := validateProductionConfig(testLogger(), "develop", "admin", "admin", "dev-password", "minio", "miniosecret", "", "")
	if err != nil {
		t.Fatalf("expected no error in develop mode, got: %v", err)
	}
}

func TestProductionConfigGuardRejectsDefaultAdminUser(t *testing.T) {
	err := validateProductionConfig(testLogger(), "v1.0.0", "admin", "Str0ng!", "s3cret", "AKID", "ASECRET", "sk_live_xxx", "whsec_xxx")
	if err == nil {
		t.Fatal("expected error for default admin user in production")
	}
	if !strings.Contains(err.Error(), "dashboard admin user") {
		t.Fatalf("error should mention dashboard admin user, got: %v", err)
	}
}

func TestProductionConfigGuardRejectsDefaultDBPassword(t *testing.T) {
	err := validateProductionConfig(testLogger(), "v1.0.0", "ops", "Str0ng!", "dev-password", "AKID", "ASECRET", "sk_live_xxx", "whsec_xxx")
	if err == nil {
		t.Fatal("expected error for default DB password in production")
	}
	if !strings.Contains(err.Error(), "database password") {
		t.Fatalf("error should mention database password, got: %v", err)
	}
}

func TestProductionConfigGuardRejectsDefaultS3Creds(t *testing.T) {
	err := validateProductionConfig(testLogger(), "v1.0.0", "ops", "Str0ng!", "s3cret", "minio", "miniosecret", "sk_live_xxx", "whsec_xxx")
	if err == nil {
		t.Fatal("expected error for default S3 credentials in production")
	}
	if !strings.Contains(err.Error(), "S3") {
		t.Fatalf("error should mention S3, got: %v", err)
	}
}

func TestProductionConfigGuardAcceptsSecureValues(t *testing.T) {
	err := validateProductionConfig(testLogger(), "v1.0.0", "ops", "Str0ng!", "pr0d-p@ss", "AKIAIOSFODNN7", "wJalrXUtnFEMI/K7MDENG", "sk_live_xxx", "whsec_xxx")
	if err != nil {
		t.Fatalf("expected no error with secure values, got: %v", err)
	}
}

func TestProductionConfigGuardWarnsEmptyStripeKeys(t *testing.T) {
	// Empty Stripe keys should NOT cause a fatal error — just a warning.
	// The app should still start; payment endpoints will fail gracefully.
	err := validateProductionConfig(testLogger(), "v1.0.0", "ops", "Str0ng!", "pr0d-p@ss", "AKIAIOSFODNN7", "wJalrXUtnFEMI/K7MDENG", "", "")
	if err != nil {
		t.Fatalf("expected no error for empty Stripe keys (should be warning only), got: %v", err)
	}
}
