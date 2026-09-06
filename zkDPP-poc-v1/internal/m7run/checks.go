package m7run

import (
	"fmt"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/audit"
	"path/filepath"
)

// ValidateReports re-reads results; it never proves, decrypts or benchmarks.
func ValidateReports(root string) error {
	committee, e := Committee(root)
	if e != nil {
		return e
	}
	fixture, e := LoadFixture(root)
	if e != nil {
		return e
	}
	events, proofs := 0, 0
	for _, g := range fixture.Groups {
		events += len(g.Events)
		proofs += len(g.Events) + len(g.Extra)
		for _, c := range append(append([]FixedCase{}, g.Events...), g.Extra...) {
			if _, _, _, e := c.Decode(); e != nil {
				return e
			}
		}
	}
	for _, name := range []string{"m7-circuit.json", "m7-anvil-gas.json", "m7-audit.json"} {
		var header struct {
			PublicKeyChecksum string
			RunCount          int
		}
		if e := Read(filepath.Join(root, "output", name), &header); e != nil {
			return e
		}
		if header.PublicKeyChecksum != committee.Public.Checksum || header.RunCount != 1 {
			return fmt.Errorf("report configuration mismatch: %s", name)
		}
	}
	var circuit struct {
		Results []struct {
			Event                     string
			Constraints, PublicInputs int
		}
		TotalProofCalls     map[string]int
		ProofReloadVerified bool
	}
	if e := Read(filepath.Join(root, "output/m7-circuit.json"), &circuit); e != nil {
		return e
	}
	if len(circuit.Results) != 8 || !circuit.ProofReloadVerified {
		return fmt.Errorf("incomplete Circuit results")
	}
	sum := 0
	for _, n := range circuit.TotalProofCalls {
		sum += n
	}
	if sum != proofs {
		return fmt.Errorf("proof count does not match fixture")
	}
	for _, r := range circuit.Results {
		found := false
		for k := audit.Entry; k <= audit.Exit; k++ {
			if k.String() != r.Event {
				continue
			}
			found = true
			l, e := Load(root, k)
			if e != nil {
				return e
			}
			if l.Manifest.Constraints != r.Constraints || l.Manifest.PublicInputs != r.PublicInputs {
				return fmt.Errorf("Circuit/artifact mismatch: %s", r.Event)
			}
			if e = CheckBinding(root, committee.Public, k); e != nil {
				return e
			}
		}
		if !found {
			return fmt.Errorf("unknown measured Event")
		}
	}
	type group struct {
		Name                                               string
		Records                                            uint64
		RootsAndPathsMatch, OriginalsRestored, StatusGates bool
	}
	type row struct {
		Group, Direction                           string
		Metrics                                    audit.Metrics
		Leaves, Entries, Terminated, Relationships int
		Expected                                   bool
	}
	for _, name := range []string{"m7-anvil-gas.json", "m7-audit.json"} {
		var r struct {
			Groups                               []group
			OriginalRestores, LedgerRuntimeBytes int
			Audits                               []row
		}
		if e := Read(filepath.Join(root, "output", name), &r); e != nil {
			return e
		}
		if r.OriginalRestores != events || r.LedgerRuntimeBytes > 24576 || len(r.Groups) != len(fixture.Groups) {
			return fmt.Errorf("incomplete EVM report")
		}
		for i, g := range r.Groups {
			if g.Name != fixture.Groups[i].Name || int(g.Records) != len(fixture.Groups[i].Events) || !g.RootsAndPathsMatch || !g.OriginalsRestored || !g.StatusGates {
				return fmt.Errorf("group validation failed")
			}
		}
		if name == "m7-audit.json" {
			expected := [][7]int{{3, 3, 2, 0, 1, 0, 4}, {5, 3, 3, 3, 0, 0, 4}, {5, 4, 3, 2, 0, 1, 4}, {4, 3, 3, 2, 0, 0, 3}, {3, 3, 2, 0, 1, 0, 3}, {4, 3, 3, 2, 0, 0, 3}, {4, 4, 2, 0, 2, 0, 4}, {3, 2, 2, 2, 0, 0, 2}, {4, 4, 1, 0, 3, 0, 6}}
			if len(r.Audits) != len(expected) {
				return fmt.Errorf("missing trace cases")
			}
			for i, v := range r.Audits {
				got := [7]int{v.Metrics.Objects, v.Metrics.UniqueRecords, v.Metrics.Decryptions, v.Leaves, v.Entries, v.Terminated, v.Relationships}
				if got != expected[i] || v.Metrics.Responses != 2*v.Metrics.Decryptions || !v.Expected {
					return fmt.Errorf("trace result mismatch %s/%s: %v", v.Group, v.Direction, got)
				}
			}
		}
	}
	return nil
}
