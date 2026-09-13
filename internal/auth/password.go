package auth

import (
	"crypto/rand"
	"fmt"
	"math/big"
)

const (
	generatedPasswordLength = 10
	passwordAlphabet        = "abcdefghjkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"
)

func generatePassword() (string, error) {
	buf := make([]byte, generatedPasswordLength)
	limit := big.NewInt(int64(len(passwordAlphabet)))

	for i := range buf {
		n, err := rand.Int(rand.Reader, limit)
		if err != nil {
			return "", fmt.Errorf("generate password: %w", err)
		}

		buf[i] = passwordAlphabet[n.Int64()]
	}

	return string(buf), nil
}
