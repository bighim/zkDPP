package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	issuecircuit "github.com/bighim/zkDPP/zkDPP-poc-v1/features/issue_claim"
	exitcircuit "github.com/bighim/zkDPP/zkDPP-poc-v1/features/m8_exit_dpp"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/artifact"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/issuepolicy"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/m8run"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/solgen"
	plonkbls "github.com/consensys/gnark/backend/plonk/bls12-381"
	"github.com/consensys/gnark/frontend"
)

type binding struct {
	Name                                string
	PublicInputs                        int
	PolicyRef, PublicKeyChecksum        string
	MinRecycledRate, MaxCarbonIntensity uint64
}

func main() {
	root := flag.String("root", ".", "project root")
	flag.Parse()
	runtime.GOMAXPROCS(8)
	if err := run(*root); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(root string) (runErr error) {
	attempt, finish, err := m8run.Begin(root, "setup", "")
	if err != nil {
		return err
	}
	defer func() { finish(runErr) }()
	if err = m8run.CaptureBaseline(root); err != nil {
		return err
	}
	committee, err := m8run.Committee(root)
	if err != nil {
		return err
	}
	type item struct {
		name    string
		circuit frontend.Circuit
		public  int
		cfg     *issuepolicy.Config
	}
	standard, strict := issuepolicy.Standard(), issuepolicy.Strict()
	items := []item{{"exit-dpp", exitcircuit.New(committee.PK), 6, nil}, {standard.Name, issuecircuit.New(standard), 2, &standard}, {strict.Name, issuecircuit.New(strict), 2, &strict}}
	manifests := map[string]artifact.Manifest{}
	for _, it := range items {
		path := m8run.Dir(root, it.name, "manifest.json")
		var loaded *artifact.Loaded
		if _, e := os.Stat(path); os.IsNotExist(e) {
			m, e := artifact.SetupAt(root, "m8", artifact.Spec{Name: it.name, Circuit: it.circuit, ExpectedPublic: it.public})
			if e != nil {
				return e
			}
			manifests[it.name] = m
		} else if e != nil {
			return e
		}
		loaded, err = m8run.Load(root, it.name)
		if err != nil {
			return err
		}
		manifests[it.name] = loaded.Manifest
		if err = artifact.ExportSolidity(filepath.Join(root, "contracts/src/generated", "m8-"+it.name, "PlonkVerifier.sol"), loaded.VK); err != nil {
			return err
		}
		b := binding{Name: it.name, PublicInputs: it.public, PublicKeyChecksum: committee.Public.Checksum}
		if it.cfg != nil {
			b.PolicyRef = it.cfg.PolicyRef.String()
			b.MinRecycledRate = it.cfg.MinRecycledRate
			b.MaxCarbonIntensity = it.cfg.MaxCarbonIntensity
			vk, ok := loaded.VK.(*plonkbls.VerifyingKey)
			if !ok {
				return fmt.Errorf("unexpected VK %T", loaded.VK)
			}
			optimized, e := solgen.ExtractOptimizedVK(vk)
			if e != nil {
				return e
			}
			if e = artifact.WriteJSON(m8run.Dir(root, it.name, "optimized-vk.json"), optimized); e != nil {
				return e
			}
		}
		if err = artifact.WriteJSON(m8run.Dir(root, it.name, "binding.json"), b); err != nil {
			return err
		}
		fmt.Printf("%s constraints=%d public=%d\n", it.name, loaded.Manifest.Constraints, loaded.Manifest.PublicInputs)
	}
	return artifact.WriteJSON(m8run.Dir(root, "setup.json"), struct {
		RunCount, Attempt int
		PublicKeyChecksum string
		Manifests         map[string]artifact.Manifest
	}{1, attempt, committee.Public.Checksum, manifests})
}
