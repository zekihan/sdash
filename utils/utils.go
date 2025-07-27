package utils

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
)

func RandString(nByte int) (string, error) {
	if nByte < 0 {
		return "", errors.New("nByte must be non-negative")
	}
	b := make([]byte, nByte)
	if nByte == 0 {
		return "", nil // Return empty string for zero length
	}
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
