package main

import "testing"

func TestLoadAdminSeedConfig(t *testing.T) {
	tests := []struct {
		name            string
		email           string
		password        string
		wantError       bool
		wantDefaultName bool
	}{
		{name: "credentials are required", wantError: true},
		{name: "email must be valid", email: "not-an-email", password: "long-enough-password", wantError: true},
		{name: "password must be strong", email: "admin@example.com", password: "short", wantError: true},
		{name: "valid credentials", email: "admin@example.com", password: "long-enough-password", wantDefaultName: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("SEED_ADMIN_EMAIL", tt.email)
			t.Setenv("SEED_ADMIN_PASSWORD", tt.password)
			t.Setenv("SEED_ADMIN_NAME", "")

			config, err := loadAdminSeedConfig()
			if (err != nil) != tt.wantError {
				t.Fatalf("loadAdminSeedConfig() error = %v, wantError %t", err, tt.wantError)
			}
			if err == nil && tt.wantDefaultName && config.Name != "System Administrator" {
				t.Errorf("default admin name = %q, want %q", config.Name, "System Administrator")
			}
		})
	}
}
