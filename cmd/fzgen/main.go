// Command go-fuzz-fill-utils automatically generates fuzz functions.
//
// See the project README for additional information:
//     https://github.com/infosecual/go-fuzz-fill-utils
package main

import (
	"os"

	"github.com/infosecual/go-fuzz-fill-utils/gen"
)

func main() {
	os.Exit(gen.FzgenMain())
}
