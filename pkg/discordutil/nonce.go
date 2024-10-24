package discordutil

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"sync/atomic"
	"time"
)

var inc = atomic.Uint64{}

func NewIncNonce() (nonce string) {
	value := inc.Add(1) - 1
	nonce = fmt.Sprintf("%d", value)
	return
}

// Returns 32 byte string
func NewNonce() (nonce string, err error) {
	timestamp := time.Now().UnixNano() / int64(time.Millisecond)
	randomBytes := make([]byte, 8)
	nonceBytes := make([]byte, 16)

	if _, err = rand.Read(randomBytes); err != nil {
		return
	}
	copy(nonceBytes, randomBytes)
	binary.BigEndian.PutUint64(nonceBytes[8:], uint64(timestamp))

	nonce = hex.EncodeToString(nonceBytes)
	return
}
