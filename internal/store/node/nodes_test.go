package node

import (
	"strings"
	"testing"
)

func TestNormalizeSecretLength(t *testing.T) {
	for _, tt := range []struct {
		length int
		valid  bool
	}{
		{length: 7},
		{length: 8, valid: true},
		{length: 128, valid: true},
		{length: 129},
	} {
		_, err := normalizeSecret(strings.Repeat("a", tt.length))
		if (err == nil) != tt.valid {
			t.Fatalf("normalizeSecret(%d chars) error = %v, valid = %t", tt.length, err, tt.valid)
		}
	}
}
