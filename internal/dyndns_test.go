package internal

import (
	"testing"
)

func TestNewDynDNSOperation(t *testing.T) {
	tests := []struct {
		name        string
		env         map[string]string
		wantDomain  string
		wantRecords []string
	}{
		{
			name: "single record",
			env: map[string]string{
				"DO_TOKEN":          "test-token",
				"DO_DOMAIN":         "example.com",
				"DO_UPDATE_RECORDS": "home",
			},
			wantDomain:  "example.com",
			wantRecords: []string{"home"},
		},
		{
			name: "multiple records",
			env: map[string]string{
				"DO_TOKEN":          "test-token",
				"DO_DOMAIN":         "example.com",
				"DO_UPDATE_RECORDS": "home,vpn,@",
			},
			wantDomain:  "example.com",
			wantRecords: []string{"home", "vpn", "@"},
		},
		{
			name: "records with surrounding whitespace are trimmed",
			env: map[string]string{
				"DO_TOKEN":          "test-token",
				"DO_DOMAIN":         "example.com",
				"DO_UPDATE_RECORDS": " home , vpn , @ ",
			},
			wantDomain:  "example.com",
			wantRecords: []string{"home", "vpn", "@"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set env vars for this test case, clean up after.
			for k, v := range tt.env {
				t.Setenv(k, v)
			}

			op, err := NewDynDNSOperation()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if op.cfg.Domain != tt.wantDomain {
				t.Errorf("Domain = %q, want %q", op.cfg.Domain, tt.wantDomain)
			}

			if len(op.cfg.UpdateRecords) != len(tt.wantRecords) {
				t.Fatalf("UpdateRecords length = %d, want %d", len(op.cfg.UpdateRecords), len(tt.wantRecords))
			}

			for i, got := range op.cfg.UpdateRecords {
				if got != tt.wantRecords[i] {
					t.Errorf("UpdateRecords[%d] = %q, want %q", i, got, tt.wantRecords[i])
				}
			}

			if op.client == nil {
				t.Error("expected non-nil godo client")
			}
		})
	}
}
