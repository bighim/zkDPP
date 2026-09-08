package main

import (
	"flag"
	"fmt"
	auditencryption "github.com/bighim/zkDPP/zkDPP-poc-v2/features/audit_encryption"
	auditprocess "github.com/bighim/zkDPP/zkDPP-poc-v2/features/audit_process_3_2"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/artifact"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/auditcrypto"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/m6b1run"
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
	attempt, finish, e := m6b1run.Begin(root, "setup", "")
	if e != nil {
		return e
	}
	defer func() { finish(err) }()
	committee, created, e := auditcrypto.EnsureCommittee(m6b1run.ArtifactDir(root, "committee"))
	if e != nil {
		return e
	}
	items := []artifact.Spec{
		{Name: m6b1run.Core, Circuit: auditencryption.New(committee.PK, 5), ExpectedPublic: 8},
		{Name: m6b1run.Process, Circuit: auditprocess.New(committee.PK), ExpectedPublic: 15},
	}
	manifests := []artifact.Manifest{}
	for _, item := range items {
		if _, e = os.Stat(m6b1run.ArtifactDir(root, item.Name, "manifest.json")); e == nil {
			if e = m6b1run.CheckBinding(root, committee.Public, item.Name); e != nil {
				return e
			}
			l, e := artifact.LoadAt(root, m6b1run.Milestone, item.Name)
			if e != nil {
				return e
			}
			manifests = append(manifests, l.Manifest)
			continue
		} else if !os.IsNotExist(e) {
			return e
		}
		m, e := artifact.SetupAt(root, m6b1run.Milestone, item)
		if e != nil {
			return e
		}
		b := m6b1run.BindingFor(committee.Public, item.Name)
		m.TreeDepth = b.TreeDepth // the standalone core has no membership tree
		if e = artifact.WriteJSON(m6b1run.ArtifactDir(root, item.Name, "manifest.json"), m); e != nil {
			return e
		}
		if e = artifact.WriteJSON(m6b1run.ArtifactDir(root, item.Name, "binding.json"), b); e != nil {
			return e
		}
		manifests = append(manifests, m)
		fmt.Printf("%s: constraints=%d public=%d\n", item.Name, m.Constraints, m.PublicInputs)
	}
	loaded, e := artifact.LoadAt(root, m6b1run.Milestone, m6b1run.Process)
	if e != nil {
		return e
	}
	if e = artifact.ExportSolidity(filepath.Join(root, "contracts", "src", "generated", "audit-process-3-2", "PlonkVerifier.sol"), loaded.VK); e != nil {
		return e
	}
	out := struct {
		RunCount          int
		Attempt           int
		Environment       m6b1run.Environment
		CommitteeCreated  bool
		PublicKeyChecksum string
		Manifests         []artifact.Manifest
	}{1, attempt, m6b1run.Env(), created, committee.Public.Checksum, manifests}
	return artifact.WriteJSON(m6b1run.ArtifactDir(root, "setup.json"), out)
}
