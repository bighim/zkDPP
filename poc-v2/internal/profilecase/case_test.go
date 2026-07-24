package profilecase

import (
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/test"
)

func TestAllBenchmarkProfilesSolve(t *testing.T) {
	assert := test.NewAssert(t)
	for _, event := range []string{"entry", "transfer", "proceed", "recall", "merge", "split", "exit"} {
		for stateLength := 0; stateLength <= 3; stateLength++ {
			c, err := Build(event, stateLength, 0)
			if err != nil {
				t.Fatalf("build %s state %d: %v", event, stateLength, err)
			}
			t.Run(c.Name, func(t *testing.T) {
				assert.SolvingSucceeded(c.Circuit, c.Assignment, test.WithCurves(ecc.BLS12_381))
			})
		}
	}
	for stateLength := 0; stateLength <= 3; stateLength++ {
		c, err := Build("process", stateLength, 3)
		if err != nil {
			t.Fatal(err)
		}
		t.Run(c.Name, func(t *testing.T) {
			assert.SolvingSucceeded(c.Circuit, c.Assignment, test.WithCurves(ecc.BLS12_381))
		})
	}
	for _, arity := range []int{1, 2} {
		c, err := Build("process", 3, arity)
		if err != nil {
			t.Fatal(err)
		}
		t.Run(c.Name, func(t *testing.T) {
			assert.SolvingSucceeded(c.Circuit, c.Assignment, test.WithCurves(ecc.BLS12_381))
		})
	}
}
