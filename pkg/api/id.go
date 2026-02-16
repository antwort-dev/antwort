package api

import (
	"crypto/rand"
	"math/big"
	"regexp"
)

const (
	idLength = 24
	charset  = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	responseIDPrefix = "resp_"
	itemIDPrefix     = "item_"
)

var (
	responseIDPattern = regexp.MustCompile(`^resp_[a-zA-Z0-9]{24}$`)
	itemIDPattern     = regexp.MustCompile(`^item_[a-zA-Z0-9]{24}$`)
)

// NewResponseID generates a new response ID with the "resp_" prefix
// followed by 24 cryptographically random alphanumeric characters.
func NewResponseID() string {
	return responseIDPrefix + randomAlphanumeric(idLength)
}

// NewItemID generates a new item ID with the "item_" prefix
// followed by 24 cryptographically random alphanumeric characters.
func NewItemID() string {
	return itemIDPrefix + randomAlphanumeric(idLength)
}

// ValidateResponseID checks whether the given string is a valid response ID
// (matches "resp_" + 24 alphanumeric characters).
func ValidateResponseID(id string) bool {
	return responseIDPattern.MatchString(id)
}

// ValidateItemID checks whether the given string is a valid item ID
// (matches "item_" + 24 alphanumeric characters).
func ValidateItemID(id string) bool {
	return itemIDPattern.MatchString(id)
}

func randomAlphanumeric(n int) string {
	max := big.NewInt(int64(len(charset)))
	b := make([]byte, n)
	for i := range b {
		idx, err := rand.Int(rand.Reader, max)
		if err != nil {
			panic("crypto/rand failed: " + err.Error())
		}
		b[i] = charset[idx.Int64()]
	}
	return string(b)
}
