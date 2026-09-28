package experiments

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"os"
	"path/filepath"
	"testing"
)

// This is a read-only source/artifact check. It generates no proof or setup.
func TestPaperSupplementSourceArtifacts(t *testing.T) {
	root := filepath.Join("..", "..")
	if _, err := os.Stat(filepath.Join(root, PaperOutput, "setup.json")); os.IsNotExist(err) {
		t.Skip("supplemental artifacts not generated")
	}
	for _, p := range paperProfiles() {
		t.Run(p.Name, func(t *testing.T) {
			k, err := paperKeys(root, p)
			if err != nil {
				t.Fatal(err)
			}
			cs, err := frontend.Compile(ecc.BLS12_381.ScalarField(), scs.NewBuilder, NewCircuit(p, defaultPK()))
			if err != nil {
				t.Fatal(err)
			}
			var b bytes.Buffer
			if _, err = cs.WriteTo(&b); err != nil {
				t.Fatal(err)
			}
			sum := sha256.Sum256(b.Bytes())
			if hex.EncodeToString(sum[:]) != k.Relation.Files["ccs.bin"] {
				t.Fatal("current source differs from setup CCS")
			}
			if cs.GetNbConstraints() != k.Relation.Constraints || cs.GetNbPublicVariables() != len(p.PublicNames()) {
				t.Fatal("relation metadata differs")
			}
		})
	}
}
