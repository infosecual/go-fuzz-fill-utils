package argexamplefuzz

// Edit if desired. Code generated by "go-fuzz-fill-utils -chain github.com/infosecual/go-fuzz-fill-utils/examples/inputs/arg-reuse".

import (
	"testing"

	argexample "github.com/infosecual/go-fuzz-fill-utils/examples/inputs/arg-reuse"
	"github.com/infosecual/go-fuzz-fill-utils/fuzzer"
)

func Fuzz_New_Chain(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
		var i int
		fz := fuzzer.NewFuzzer(data)
		fz.Fill(&i)

		target := argexample.New(i)

		steps := []fuzzer.Step{
			{
				Name: "Fuzz_PanicOnArgReuse_Step1",
				Func: func(a int64, b int64) {
					target.Step1(a, b)
				},
			},
			{
				Name: "Fuzz_PanicOnArgReuse_Step2",
				Func: func(a int64, b int64) {
					target.Step2(a, b)
				},
			},
		}

		// Execute a specific chain of steps, with the count, sequence and arguments controlled by fz.Chain
		fz.Chain(steps)
	})
}
