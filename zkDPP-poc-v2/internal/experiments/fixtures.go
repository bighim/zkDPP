package experiments

import (
	"fmt"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/solgen"
	bls "github.com/consensys/gnark/backend/plonk/bls12-381"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// ExportFoundry uses the successful development lifecycle's real proofs.
func ExportFoundry(root string) error {
	var paths []string
	if env, e := loadEnvelope(root, 1); e == nil {
		for _, r := range measureRows(env) {
			if r.Run == 0 {
				paths = append(paths, r.ProofArtifact)
			}
		}
	}
	if len(paths) == 0 {
		candidates, _ := filepath.Glob(filepath.Join(root, ArtifactDir, "attempts/smoke-1-*.json"))
		sort.Strings(candidates)
		if len(candidates) > 0 {
			var env Envelope
			if readJSON(candidates[len(candidates)-1], &env) == nil {
				for _, r := range measureRows(env) {
					if r.Run == 0 && r.ProofArtifact != "" {
						paths = append(paths, r.ProofArtifact)
					}
				}
			}
		}
	}
	var e error
	if len(paths) == 0 {
		paths, e = filepath.Glob(filepath.Join(root, ArtifactDir, "proofs/exp1-lifecycle-*.json"))
	}
	if e != nil {
		return e
	}
	sort.Strings(paths)
	if len(paths) != 14 {
		return fmt.Errorf("expected 14 archived lifecycle proof frames, got %d", len(paths))
	}
	var frames []map[string]any
	for _, path := range paths {
		if !filepath.IsAbs(path) {
			path = filepath.Join(root, path)
		}
		var f map[string]any
		if e = readJSON(path, &f); e != nil {
			return e
		}
		frames = append(frames, f)
	}
	policies := []map[string]any{}
	for _, name := range []string{"process-2-1-audit", "issue-audit"} {
		k, e := LoadKeys(root, name)
		if e != nil {
			return e
		}
		opt, e := solgen.ExtractOptimizedVK(k.VK.(*bls.VerifyingKey))
		if e != nil {
			return e
		}
		idx := 0
		for i, p := range Profiles() {
			if p.Name == name {
				idx = i
			}
		}
		policies = append(policies, map[string]any{"ref": k.Relation.Profile.PolicyRef.String(), "hash": "0x" + opt.SHA256, "verifierIndex": idx})
	}
	var setup SetupResult
	if e = readJSON(filepath.Join(root, OutputDir, "setup.json"), &setup); e != nil {
		return e
	}
	target := filepath.Join(root, "contracts/experiments/test/fixtures/lifecycle.json")
	if _, e = os.Stat(target); e == nil {
		backup := filepath.Join(root, ArtifactDir, "attempts", fmt.Sprintf("foundry-fixture-%d.json", time.Now().UnixNano()))
		if e = os.Rename(target, backup); e != nil {
			return e
		}
	}
	return writeJSON(target, map[string]any{"frames": frames, "policies": policies, "source": "successful lifecycle warmup; reused for correctness, not an additional timing sample", "srsChecksum": setup.CanonicalChecksum})
}
