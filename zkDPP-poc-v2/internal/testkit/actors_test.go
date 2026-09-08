package testkit_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/testkit"
)

func actorPath() string {
	return filepath.Join("..", "..", "testdata", "common", "actors-v1.json")
}

func TestActorsFixture(t *testing.T) {
	actors, err := testkit.LoadActors(actorPath())
	if err != nil {
		t.Fatal(err)
	}
	if len(actors) != 5 {
		t.Fatalf("actors=%d, want 5", len(actors))
	}
	actor, err := testkit.ByID(actors, "actor-3")
	if err != nil {
		t.Fatal(err)
	}
	if actor.ID != "actor-3" {
		t.Fatalf("got actor %q", actor.ID)
	}
}

func TestInvalidActorsFixtureFailsAtomically(t *testing.T) {
	source, err := os.ReadFile(actorPath())
	if err != nil {
		t.Fatal(err)
	}
	mutations := map[string]string{
		"metadata":           strings.Replace(string(source), "zkDPP-fixed-actors-v1", "wrong-profile", 1),
		"zero secret":        strings.Replace(string(source), `"skOwner": "11"`, `"skOwner": "0"`, 1),
		"duplicate id":       strings.Replace(string(source), `"id": "actor-5"`, `"id": "actor-4"`, 1),
		"duplicate secret":   strings.Replace(string(source), `"skOwner": "55"`, `"skOwner": "44"`, 1),
		"address mismatch":   strings.Replace(string(source), "49991025685611962253864790410137325962643753898623019661476626077140422570340", "1", 1),
		"unknown json field": strings.Replace(string(source), `"profile":`, `"extra": true, "profile":`, 1),
	}
	for name, data := range mutations {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "actors.json")
			if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := testkit.LoadActors(path); err == nil {
				t.Fatal("invalid fixture accepted")
			}
		})
	}
}
