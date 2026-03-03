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
	fileIDPrefix         = "file_"
	batchIDPrefix        = "batch_"
	conversationIDPrefix = "conv_"
)

var (
	responseIDPattern     = regexp.MustCompile(`^resp_[a-zA-Z0-9]{24}$`)
	itemIDPattern         = regexp.MustCompile(`^item_[a-zA-Z0-9]{24}$`)
	fileIDPattern         = regexp.MustCompile(`^file_[a-zA-Z0-9]{24}$`)
	batchIDPattern        = regexp.MustCompile(`^batch_[a-zA-Z0-9]{24}$`)
	conversationIDPattern = regexp.MustCompile(`^conv_[a-zA-Z0-9]{24}$`)
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

// NewFileID generates a new file ID with the "file_" prefix
// followed by 24 cryptographically random alphanumeric characters.
func NewFileID() string {
	return fileIDPrefix + randomAlphanumeric(idLength)
}

// NewBatchID generates a new batch ID with the "batch_" prefix
// followed by 24 cryptographically random alphanumeric characters.
func NewBatchID() string {
	return batchIDPrefix + randomAlphanumeric(idLength)
}

// NewConversationID generates a new conversation ID with the "conv_" prefix
// followed by 24 cryptographically random alphanumeric characters.
func NewConversationID() string {
	return conversationIDPrefix + randomAlphanumeric(idLength)
}

// ValidateConversationID checks whether the given string is a valid conversation ID.
func ValidateConversationID(id string) bool {
	return conversationIDPattern.MatchString(id)
}

// ValidateFileID checks whether the given string is a valid file ID
// (matches "file_" + 24 alphanumeric characters).
func ValidateFileID(id string) bool {
	return fileIDPattern.MatchString(id)
}

// ValidateBatchID checks whether the given string is a valid batch ID
// (matches "batch_" + 24 alphanumeric characters).
func ValidateBatchID(id string) bool {
	return batchIDPattern.MatchString(id)
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
