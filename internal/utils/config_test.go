package utils

import "testing"

func TestValidateProductionConfig(t *testing.T) {
	tests := []struct {
		name           string
		env            string
		dbURL          string
		jwtSecret      string
		allowedOrigins string
		wantErr        bool
	}{
		{
			name:    "development keeps local defaults",
			env:     "development",
			wantErr: false,
		},
		{
			name:           "production requires database",
			env:            "production",
			jwtSecret:      "a-long-random-secret-that-is-at-least-32-chars",
			allowedOrigins: "https://app.example.com",
			wantErr:        true,
		},
		{
			name:           "production requires strong non-default jwt secret",
			env:            "production",
			dbURL:          "postgres://user:pass@db.example.com/app",
			jwtSecret:      "default-dev-secret-key-change-in-production-32chars",
			allowedOrigins: "https://app.example.com",
			wantErr:        true,
		},
		{
			name:           "production rejects wildcard origins",
			env:            "production",
			dbURL:          "postgres://user:pass@db.example.com/app",
			jwtSecret:      "a-long-random-secret-that-is-at-least-32-chars",
			allowedOrigins: "https://app.example.com, *",
			wantErr:        true,
		},
		{
			name:           "production accepts explicit configuration",
			env:            "production",
			dbURL:          "postgres://user:pass@db.example.com/app",
			jwtSecret:      "a-long-random-secret-that-is-at-least-32-chars",
			allowedOrigins: "https://app.example.com,https://admin.example.com",
			wantErr:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateProductionConfig(tt.env, tt.dbURL, tt.jwtSecret, tt.allowedOrigins)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateProductionConfig() error = %v, wantErr %t", err, tt.wantErr)
			}
		})
	}
}
