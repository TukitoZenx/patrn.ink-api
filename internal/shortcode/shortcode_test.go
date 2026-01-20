package shortcode

import "testing"

func TestEncode(t *testing.T) {
	tests := []struct {
		name     string
		input    uint64
		expected string
	}{
		{"Zero", 0, "a"},
		{"One", 1, "b"},
		{"Small", 10, "k"},
		{"Medium", 123, "b9"},
		{"Large", 999999, "emjb"},
		{"Very Large", 9999999, "P7Ct"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Encode(tt.input)
			if result != tt.expected {
				t.Errorf("Encode(%d) = %s; want %s", tt.input, result, tt.expected)
			}
		})
	}
}

func TestIsValidCustomCode(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		expected bool
	}{
		{"Valid short", "abc", true},
		{"Valid long", "my-custom-link-123", true},
		{"Valid with underscore", "my_link", true},
		{"Too short", "ab", false},
		{"Too long", "this-is-too-long-for-a-custom-code", false},
		{"Invalid chars", "my link!", false},
		{"Valid numbers", "123", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsValidCustomCode(tt.code)
			if result != tt.expected {
				t.Errorf("IsValidCustomCode(%s) = %v; want %v", tt.code, result, tt.expected)
			}
		})
	}
}

func BenchmarkEncode(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Encode(uint64(i))
	}
}
