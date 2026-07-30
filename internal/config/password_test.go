package config

import "testing"

func TestValidateAdminPasswordMinimumLength(t *testing.T) {
	if err := ValidateAdminPassword("12345678"); err != nil {
		t.Fatalf("ValidateAdminPassword() error = %v", err)
	}
	if err := ValidateAdminPassword("1234567"); err == nil {
		t.Fatal("ValidateAdminPassword() succeeded with fewer than 8 characters")
	}
}
