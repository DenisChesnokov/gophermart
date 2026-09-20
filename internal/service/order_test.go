package service

import "testing"

func TestValidateLuhn(t *testing.T) {
	tests := []struct {
		name   string
		number string
		valid  bool
	}{
		{"valid 79927398713", "79927398713", true},
		{"valid 12345678903", "12345678903", true},
		{"valid 4242424242424242", "4242424242424242", true},
		{"invalid 123456789", "123456789", false},
		{"invalid 1234567890", "1234567890", false},
		{"invalid 79927398712", "79927398712", false},
		{"empty string", "", false},
		{"non-numeric abc", "abc", false},
		{"non-numeric 12abc", "12abc", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ValidateLuhn(tt.number)
			if got != tt.valid {
				t.Errorf("ValidateLuhn(%q) = %v, want %v", tt.number, got, tt.valid)
			}
		})
	}
}
