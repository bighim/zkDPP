package deterministic

import (
	"crypto/sha256"
	"encoding/binary"
)

// Reader is a reproducible byte stream for test vectors only. It is not a cryptographic RNG.
type Reader struct {
	seed    []byte
	counter uint64
	buffer  []byte
}

func New(seed string) *Reader {
	return &Reader{seed: []byte(seed)}
}

func (r *Reader) Read(dst []byte) (int, error) {
	written := 0
	for written < len(dst) {
		if len(r.buffer) == 0 {
			var counter [8]byte
			binary.BigEndian.PutUint64(counter[:], r.counter)
			r.counter++
			hash := sha256.New()
			hash.Write(r.seed)
			hash.Write(counter[:])
			r.buffer = hash.Sum(nil)
		}
		n := copy(dst[written:], r.buffer)
		r.buffer = r.buffer[n:]
		written += n
	}
	return written, nil
}
