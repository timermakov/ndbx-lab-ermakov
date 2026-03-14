package session

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

const (
	idBytes     = 16 // 128 bits
	idHexLength = idBytes * 2
	hexAlphabet = "0123456789abcdef"
)

// GenerateID generates a new cryptographically secure session identifier.
func GenerateID() (string, error) {
	buf := make([]byte, idBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate session id: %w", err)
	}

	return hex.EncodeToString(buf), nil
}

// ValidateID checks that the provided id is a valid hex-encoded session identifier.
func ValidateID(id string) bool {
	if len(id) != idHexLength {
		return false
	}

	for i := 0; i < len(id); i++ {
		c := id[i]
		if !isHexChar(c) {
			return false
		}
	}

	return true
}

func isHexChar(c byte) bool {
	for i := 0; i < len(hexAlphabet); i++ {
		if c == hexAlphabet[i] {
			return true
		}
	}
	if c >= 'A' && c <= 'F' {
		return true
	}
	return false
}
