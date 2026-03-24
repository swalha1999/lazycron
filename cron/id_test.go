package cron

import (
	"testing"
)

func TestGenerateID_Length(t *testing.T) {
	id := GenerateID()
	if len(id) != 8 {
		t.Errorf("GenerateID() = %q, want 8 characters", id)
	}
}

func TestGenerateID_Hex(t *testing.T) {
	id := GenerateID()
	for _, c := range id {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("GenerateID() = %q, contains non-hex character %q", id, string(c))
		}
	}
}

func TestGenerateID_Unique(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		id := GenerateID()
		if seen[id] {
			t.Fatalf("duplicate ID after %d iterations: %q", i, id)
		}
		seen[id] = true
	}
}

func TestIsIDChar(t *testing.T) {
	valid := "abcdefghijklmnopqrstuvwxyz0123456789_-"
	for _, c := range valid {
		if !IsIDChar(byte(c)) {
			t.Errorf("IsIDChar(%q) = false, want true", string(c))
		}
	}
	invalid := "ABCDEFGHIJKLMNOPQRSTUVWXYZ ./@#$%^&*()+="
	for _, c := range invalid {
		if IsIDChar(byte(c)) {
			t.Errorf("IsIDChar(%q) = true, want false", string(c))
		}
	}
}

func TestValidateID_Valid(t *testing.T) {
	valid := []string{
		"deadbeef",
		"db-backup",
		"salati_cleanup",
		"salati-cleanup",
		"a",
		"my-job-123",
		"a1b2c3d4",
	}
	for _, id := range valid {
		if err := ValidateID(id); err != nil {
			t.Errorf("ValidateID(%q) = %v, want nil", id, err)
		}
	}
}

func TestValidateID_Invalid(t *testing.T) {
	tests := []struct {
		id   string
		desc string
	}{
		{"", "empty"},
		{"DB-BACKUP", "uppercase"},
		{"my job", "space"},
		{"my.job", "dot"},
		{"@id", "special char"},
		{"-starts-with-hyphen", "leading hyphen"},
		{"ends-with-hyphen-", "trailing hyphen"},
		{"_leading", "leading underscore"},
		{"trailing_", "trailing underscore"},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			if err := ValidateID(tt.id); err == nil {
				t.Errorf("ValidateID(%q) = nil, want error", tt.id)
			}
		})
	}
}
