package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"

	"github.com/bighim/zkDPP/sahai-poc/internal/document"
)

type vector struct {
	Profile              string `json:"hashProfile"`
	Root                 string `json:"root"`
	AttributeCommitment  string `json:"attributeCommitment"`
	PublicInputs         int    `json:"digestPublicInputs"`
	OpeningVerified      bool   `json:"openingVerified"`
	SaltMutationRejected bool   `json:"saltMutationRejected"`
}

func main() {
	doc := document.CanonicalDocument()
	results := make([]vector, 0, 2)
	for _, profile := range []document.Profile{document.Poseidon2, document.SHA256} {
		tree := document.Build(profile, doc)
		root := tree.Root()
		opening, err := tree.Open(doc, 2)
		if err != nil {
			fatal(err)
		}
		valid := document.RootFromOpening(profile, opening) == root
		commitment := document.AttributeCommitment(profile, doc.Attributes[0], doc.Salts[0])
		mutatedSalt := doc.Salts[0]
		mutatedSalt[15] ^= 1
		rejected := document.AttributeCommitment(profile, doc.Attributes[0], mutatedSalt) != commitment
		if !valid || !rejected {
			fatal(fmt.Errorf("%s golden vector failed", profile))
		}
		results = append(results, vector{Profile: string(profile), Root: "0x" + hex.EncodeToString(root[:]), AttributeCommitment: "0x" + hex.EncodeToString(commitment[:]), PublicInputs: len(document.DigestPublic(profile, root)), OpeningVerified: valid, SaltMutationRejected: rejected})
	}
	if results[0].Root == results[1].Root {
		fatal(fmt.Errorf("hash profiles produced the same root"))
	}
	raw, err := json.MarshalIndent(map[string]any{"schemaVersion": 1, "vectors": results, "crossProfileRootDifferent": true}, "", "  ")
	if err != nil {
		fatal(err)
	}
	if err = os.WriteFile("testdata/hash-golden-vectors.json", append(raw, '\n'), 0644); err != nil {
		fatal(err)
	}
	fmt.Println(string(raw))
}
func fatal(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
