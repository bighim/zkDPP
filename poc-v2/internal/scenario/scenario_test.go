package scenario_test

import (
	"reflect"
	"testing"

	"github.com/bighim/zkDPP/poc-v2/internal/protocol"
	"github.com/bighim/zkDPP/poc-v2/internal/scenario"
)

func TestCanonicalScenarioArithmetic(t *testing.T) {
	d := load(t)
	expected := map[string]protocol.State{
		"foil-first":        {16799999994, 161500000170, 10000000000},
		"foil-scrap-first":  {1200000006, 8499999830, 0},
		"foil-second":       {8399999997, 152000000160, 0},
		"foil-scrap-second": {600000003, 7999999840, 0},
		"merged-foil":       {25199999991, 313500000330, 10000000000},
		"cell":              {274680000298, 867825001228, 70000000000},
		"cell-scrap":        {24415999694, 36539999099, 0},
		"waste":             {6103999999, 9135000003, 0},
		"cell-exit":         {164808000179, 520695000737, 42000000000},
		"cell-held":         {109872000119, 347130000491, 28000000000},
	}
	for name, want := range expected {
		if got := d.Notes[name].State; got != want {
			t.Errorf("%s State=%v, want %v", name, got, want)
		}
	}
	if got := d.Remainders["split:cell:output2"]; got != [3]uint64{200, 200, 0} {
		t.Errorf("split remainder=%v", got)
	}
	if got := d.Remainders["process:cell:output2"]; got != [3]uint64{80000009, 699999670, 0} {
		t.Errorf("cell process output2 remainder=%v", got)
	}
	if got := d.Remainders["process:cell:output3"]; got != [3]uint64{820000000, 300000000, 0} {
		t.Errorf("cell process output3 remainder=%v", got)
	}
	if d.MT.Count() != 28 || d.RVMT.Count() != 7 {
		t.Errorf("final tree counts MT=%d rvMT=%d", d.MT.Count(), d.RVMT.Count())
	}
	foilFirst, foilSecond := d.Notes["foil-first"], d.Notes["foil-second"]
	if !foilFirst.DocumentHash.Equal(&foilSecond.DocumentHash) {
		t.Error("foil DocumentHash values differ")
	}
	if foilFirst.Commitment.Equal(&foilSecond.Commitment) {
		t.Error("foil commitments unexpectedly match")
	}
}

func TestEntryStatesAreWholeNanoUnits(t *testing.T) {
	d := load(t)
	for _, entry := range d.Raw.Entries {
		note := d.Notes[entry.ID]
		for k, value := range note.State {
			if value%1_000_000_000 != 0 {
				t.Errorf("%s state[%d]=%d is not a whole nano unit", entry.ID, k, value)
			}
		}
	}
}

func TestCanonicalManifest(t *testing.T) {
	manifests := protocol.CanonicalManifests()
	wantCounts := []int{1, 6, 4, 4, 7, 5, 26, 3}
	if len(manifests) != 8 {
		t.Fatalf("manifest count=%d", len(manifests))
	}
	for i, manifest := range manifests {
		if int(manifest.ID) != i || manifest.Event != protocol.EventNames[i] {
			t.Errorf("manifest %d identity mismatch: %+v", i, manifest)
		}
		if len(manifest.Inputs) != wantCounts[i] {
			t.Errorf("%s public inputs=%d, want %d", manifest.Event, len(manifest.Inputs), wantCounts[i])
		}
	}
	wantProcessPrefix := []string{"m", "n", "p_0", "p_1", "p_2", "A_0_0"}
	if !reflect.DeepEqual(manifests[6].Inputs[:6], wantProcessPrefix) {
		t.Errorf("process prefix=%v", manifests[6].Inputs[:6])
	}
}

func TestCanonicalTreePaths(t *testing.T) {
	d := load(t)
	for _, name := range []string{"proceed-cathode", "proceed-anode", "merged-foil"} {
		note := d.Notes[name]
		path, err := d.MT.Path(note.Index)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(len(path.Siblings), protocol.TreeDepth) {
			t.Fatalf("path depth=%d", len(path.Siblings))
		}
	}
}

func load(t *testing.T) *scenario.Derived {
	t.Helper()
	d, err := scenario.LoadCanonical("../..")
	if err != nil {
		t.Fatal(err)
	}
	return d
}
