package cron

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// GenerateID returns a random 8-character lowercase hex string
// suitable for use as a unique job identifier.
func GenerateID() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return hex.EncodeToString(b)
}

// IsIDChar reports whether c is a valid job ID character: [a-z0-9_-].
func IsIDChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_' || c == '-'
}

// ValidateID checks that id is a valid job identifier.
// Valid IDs contain only [a-z0-9_-], must not be empty, must not start or
// end with - or _, and must be at most 64 characters.
func ValidateID(id string) error {
	if id == "" {
		return fmt.Errorf("job ID must not be empty")
	}
	if len(id) > 64 {
		return fmt.Errorf("job ID %q exceeds 64 characters", id)
	}
	if id[0] == '-' || id[0] == '_' {
		return fmt.Errorf("job ID %q must not start with %q", id, string(id[0]))
	}
	if id[len(id)-1] == '-' || id[len(id)-1] == '_' {
		return fmt.Errorf("job ID %q must not end with %q", id, string(id[len(id)-1]))
	}
	for i := 0; i < len(id); i++ {
		if !IsIDChar(id[i]) {
			return fmt.Errorf("job ID %q contains invalid character %q (allowed: a-z, 0-9, _, -)", id, string(id[i]))
		}
	}
	return nil
}
