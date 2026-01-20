package shortcode

import "regexp"

const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// Encode converts an integer to a base62 string using a stable alphabet.
func Encode(input uint64) string {
	if input == 0 {
		return string(alphabet[0])
	}

	var encoded []byte
	for input > 0 {
		remainder := input % uint64(len(alphabet))
		encoded = append(encoded, alphabet[remainder])
		input /= uint64(len(alphabet))
	}

	// Reverse in place
	for i, j := 0, len(encoded)-1; i < j; i, j = i+1, j-1 {
		encoded[i], encoded[j] = encoded[j], encoded[i]
	}

	return string(encoded)
}

// IsValidCustomCode validates custom short codes.
func IsValidCustomCode(code string) bool {
	if len(code) < 3 || len(code) > 20 {
		return false
	}
	matched, _ := regexp.MatchString("^[a-zA-Z0-9_-]+$", code)
	return matched
}
