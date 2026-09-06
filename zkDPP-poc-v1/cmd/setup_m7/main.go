package main

import (
	"flag"
	"fmt"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/artifact"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/audit"
	b1 "github.com/bighim/zkDPP/zkDPP-poc-v1/internal/m6b1run"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/m7case"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/m7run"
	"os"
	"path/filepath"
	"runtime"
)

func main() {
	root := flag.String("root", ".", "project root")
	flag.Parse()
	runtime.GOMAXPROCS(8)
	if e := run(*root); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
func run(root string) (err error) {
	attempt, finish, e := m7run.Begin(root, "setup", "")
	if e != nil {
		return e
	}
	defer func() { finish(err) }()
	committee, e := m7run.Committee(root)
	if e != nil {
		return e
	}
	defs := m7case.Definitions(committee.PK)
	manifests := map[string]artifact.Manifest{}
	for k := audit.Entry; k <= audit.Exit; k++ {
		if k == audit.Process {
			if e = m7run.CheckBinding(root, committee.Public, k); e != nil {
				return e
			}
			l, e := m7run.Load(root, k)
			if e != nil {
				return e
			}
			manifests[k.String()] = l.Manifest
			continue
		}
		name := m7run.Name(k)
		binding := m7run.Bind(committee.Public, k)
		var manifest artifact.Manifest
		if _, e = os.Stat(m7run.Dir(root, name, "manifest.json")); e == nil {
			if e = m7run.CheckBinding(root, committee.Public, k); e != nil {
				return e
			}
			l, e := m7run.Load(root, k)
			if e != nil {
				return e
			}
			manifest = l.Manifest
		} else if os.IsNotExist(e) {
			manifest, e = artifact.SetupAt(root, "m7", artifact.Spec{Name: name, Circuit: defs[k], ExpectedPublic: binding.PublicInputs})
			if e != nil {
				return e
			}
			if k == audit.Entry {
				manifest.TreeDepth = 0
			}
			if e = artifact.WriteJSON(m7run.Dir(root, name, "manifest.json"), manifest); e != nil {
				return e
			}
			if e = artifact.WriteJSON(m7run.Dir(root, name, "binding.json"), binding); e != nil {
				return e
			}
		} else {
			return e
		}
		loaded, e := m7run.Load(root, k)
		if e != nil {
			return e
		}
		if e = artifact.ExportSolidity(filepath.Join(root, "contracts/src/generated", "m7-"+k.String(), "PlonkVerifier.sol"), loaded.VK); e != nil {
			return e
		}
		manifests[k.String()] = manifest
		fmt.Printf("%s: constraints=%d public=%d\n", k, manifest.Constraints, manifest.PublicInputs)
	}
	return artifact.WriteJSON(m7run.Dir(root, "setup.json"), struct {
		RunCount, Attempt int
		Environment       b1.Environment
		PublicKeyChecksum string
		Manifests         map[string]artifact.Manifest
		ReusedProcess     bool
	}{1, attempt, b1.Env(), committee.Public.Checksum, manifests, true})
}
