package infra

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

type SystemClock struct{}

func NewSystemClock() SystemClock {
	return SystemClock{}
}

func (SystemClock) Now() time.Time {
	return time.Now().UTC()
}

type CryptoIDGenerator struct{}

func NewCryptoIDGenerator() CryptoIDGenerator {
	return CryptoIDGenerator{}
}

func (CryptoIDGenerator) NewID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("generate event id: %w", err)
	}
	return "evt_" + hex.EncodeToString(value[:]), nil
}
