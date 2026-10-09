package seed

import (
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"math/rand/v2"
)

type Source struct{ value uint64 }

func New(value uint64) Source { return Source{value: value} }

func Pick() uint64 {
	for {
		if v := rand.Uint64(); v != 0 { //nolint:gosec // seeds mock data, which must be reproducible, not secret
			return v
		}
	}
}

func (s Source) Value() uint64 { return s.value }

func (s Source) Stream(parts ...string) *rand.Rand {
	h := fnv.New64a()
	for _, p := range parts {
		_, _ = h.Write([]byte(p))
		_, _ = h.Write([]byte{0})
	}
	return rand.New(rand.NewPCG(s.value, h.Sum64())) //nolint:gosec // seeds mock data, which must be reproducible, not secret
}

func UUID(r *rand.Rand) string {
	var b [16]byte
	binary.BigEndian.PutUint64(b[:8], r.Uint64())
	binary.BigEndian.PutUint64(b[8:], r.Uint64())
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
