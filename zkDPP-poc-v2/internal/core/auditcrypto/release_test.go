package auditcrypto

import (
	"testing"
)

func TestReleaseContextBinding(t *testing.T) {
	pk, s := testKeys(t)
	p, e := PackageFromShares("release-session", pk, s)
	if e != nil {
		t.Fatal(e)
	}
	r := []ReleasedShare{{p.Profile, p.SessionID, PackageChecksum(p), s[0]}, {p.Profile, p.SessionID, PackageChecksum(p), s[1]}}
	for _, field := range []string{"profile", "session", "checksum", "public"} {
		bad := append([]ReleasedShare{}, r...)
		q := p
		switch field {
		case "profile":
			bad[0].Profile = "old"
		case "session":
			bad[0].SessionID = "other"
		case "checksum":
			bad[0].PackageChecksum = "wrong"
		case "public":
			q.PublicShares[0] = Generator()
		}
		if _, e = RecoverReleased(q, bad); e == nil {
			t.Fatalf("accepted wrong %s", field)
		}
	}
}
