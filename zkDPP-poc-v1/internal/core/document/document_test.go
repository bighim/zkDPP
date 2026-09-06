package document_test

import (
	"bytes"
	"testing"

	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/document"
)

func TestEncodingUsesNFCAndFieldLengths(t *testing.T) {
	a, err := document.Encode(document.DocumentInfo{ProductName: "e\u0301", LotID: "lot"})
	if err != nil {
		t.Fatal(err)
	}
	b, err := document.Encode(document.DocumentInfo{ProductName: "é", LotID: "lot"})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a, b) {
		t.Fatalf("NFC-equivalent strings differ: %x != %x", a, b)
	}
	c, err := document.Encode(document.DocumentInfo{ProductName: "é", LotID: "lo"})
	if err != nil {
		t.Fatal(err)
	}
	d, err := document.Encode(document.DocumentInfo{ProductName: "élo", LotID: ""})
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(c, d) {
		t.Fatal("length-delimited fields collided")
	}
}

func TestHashIsDeterministic(t *testing.T) {
	info := document.DocumentInfo{ProductName: "Aluminum", LotID: "LOT-M1-001"}
	a, err := document.Hash(info)
	if err != nil {
		t.Fatal(err)
	}
	b, err := document.Hash(info)
	if err != nil {
		t.Fatal(err)
	}
	if !a.Equal(&b) {
		t.Fatal("DocumentHash is not deterministic")
	}
}
