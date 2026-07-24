package zkhash_test

import (
	"bytes"
	"testing"

	"github.com/bighim/zkDPP/poc-v2/internal/zkhash"
)

func TestDocumentInfoEncodingIsLengthDelimitedAndNFC(t *testing.T) {
	a, err := zkhash.EncodeDocumentInfo("e\u0301", "lot", "kg")
	if err != nil {
		t.Fatal(err)
	}
	b, err := zkhash.EncodeDocumentInfo("é", "lot", "kg")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a, b) {
		t.Fatalf("NFC-equivalent strings encoded differently: %x != %x", a, b)
	}
	c, err := zkhash.EncodeDocumentInfo("é", "lo", "tkg")
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(a, c) {
		t.Fatal("length-delimited fields collided")
	}
}

func TestPoseidonHashAndCompressAreDeterministic(t *testing.T) {
	one := zkhash.Element(1)
	two := zkhash.Element(2)
	if got, want := zkhash.Hash(one, two), zkhash.Hash(one, two); !got.Equal(&want) {
		t.Fatal("Poseidon2 hash is not deterministic")
	}
	if got, want := zkhash.Compress(one, two), zkhash.Compress(one, two); !got.Equal(&want) {
		t.Fatal("Poseidon2 compression is not deterministic")
	}
}
