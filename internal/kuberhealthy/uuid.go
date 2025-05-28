package kuberhealthy

import (
	"fmt"
	"time"
)

func makeUUID() [16]byte {
	var uuid [16]byte
	_, err := fmt.Sscanf(fmt.Sprintf("%016x", newUUIDSeed()), "%x", &uuid)
	if err != nil {
		panic("failed to generate UUID seed")
	}
	return uuid
}

func newUUIDSeed() int64 {
	return time.Now().UnixNano()
}
