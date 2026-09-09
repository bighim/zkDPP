// M1 reports are retained in commit 8560fd3.
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "M1 reports are preserved. Use make benchmark-m1-hotfix or make check-m1-hotfix.")
	os.Exit(1)
}
