package benchmarks

import (
	"encoding/hex"
	"math/rand"
	"time"
)

func newUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

var seededRand *rand.Rand = rand.New(rand.NewSource(time.Now().UnixNano()))

func newRandCharSetString(length int) string {
	charset := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}
