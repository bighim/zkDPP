package common

import "github.com/consensys/gnark/frontend"

const TreeDepth = 32

type MerklePath struct {
	Index    frontend.Variable
	Siblings [TreeDepth]frontend.Variable
}

func AssertMembership(api frontend.API, root, leaf frontend.Variable, path MerklePath) error {
	bits := api.ToBinary(path.Index, TreeDepth)
	current := leaf
	for level := 0; level < TreeDepth; level++ {
		left := api.Select(bits[level], path.Siblings[level], current)
		right := api.Select(bits[level], current, path.Siblings[level])
		parent, err := Compress(api, left, right)
		if err != nil {
			return err
		}
		current = parent
	}
	api.AssertIsEqual(current, root)
	return nil
}
