package hotfix

import (
	"encoding/json"
	"fmt"
	zkhash "github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/hash"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/testkit"
	"os"
	"path/filepath"
	"reflect"
)

func Prepare(root string) error {
	actors, e := testkit.LoadActors(filepath.Join(root, "testdata/common/actors-v1.json"))
	if e != nil {
		return e
	}
	var raw map[string]any
	b, e := os.ReadFile(filepath.Join(root, "testdata/common/actors-v1.json"))
	if e != nil {
		return e
	}
	if e = json.Unmarshal(b, &raw); e != nil {
		return e
	}
	raw["profile"] = "zkDPP-fixed-actors-v2"
	raw["ownerTag"] = zkhash.OwnerTagString
	rows := raw["actors"].([]any)
	for i, a := range actors {
		rows[i].(map[string]any)["address"] = a.Address.String()
	}
	fmt.Println("PolicyRefTag", zkhash.PolicyRefTag.String())
	if b, e := os.ReadFile(filepath.Join(root, "testdata/common/actors-v2.json")); e == nil {
		var prior map[string]any
		if e = json.Unmarshal(b, &prior); e != nil {
			return e
		}
		if !reflect.DeepEqual(prior, raw) {
			return fmt.Errorf("actor v2 profile changed")
		}
		return nil
	}
	return Write(filepath.Join(root, "testdata/common/actors-v2.json"), raw)
}
