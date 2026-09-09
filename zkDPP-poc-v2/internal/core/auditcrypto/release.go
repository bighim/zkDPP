package auditcrypto

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
)

type ReleasedShare struct {
	Profile, SessionID, PackageChecksum string
	Share                               Share
}

func PackageChecksum(p ExternalKeyPackage) string {
	b, _ := json.Marshal(p)
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}
func RecoverReleased(p ExternalKeyPackage, r []ReleasedShare) (*big.Int, error) {
	if len(r) != 2 {
		return nil, fmt.Errorf("exactly two releases required")
	}
	shares := make([]Share, 2)
	defer func() {
		for _, s := range shares {
			if s.Value != nil {
				s.Value.SetInt64(0)
			}
		}
	}()
	for i, v := range r {
		if v.Profile != p.Profile || v.SessionID != p.SessionID || v.PackageChecksum != PackageChecksum(p) {
			return nil, fmt.Errorf("release context mismatch")
		}
		if e := validateShare(v.Share); e != nil {
			return nil, e
		}
		shares[i] = Share{ID: v.Share.ID, Value: new(big.Int).Set(v.Share.Value)}
	}
	return RecoverMasterKey(p, shares)
}
