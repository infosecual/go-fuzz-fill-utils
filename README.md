# go-fuzz-fill-utils

go-fuzz-fill-utils auto-generates fuzzing wrappers for Go 1.18, optionally finds problematic API call sequences and concurrency bugs, can automatically wire together outputs & inputs across API calls, and supports fuzzing complex types such as structs, maps and common interfaces.

## Why?

Fewer bugs, happy Gophers.

Modern [fuzzing](https://en.wikipedia.org/wiki/Fuzzing) has had a large amount of [success](https://events.linuxfoundation.org/wp-content/uploads/2017/11/Syzbot-and-the-Tale-of-Thousand-Kernel-Bugs-Dmitry-Vyukov-Google.pdf) and can be almost [eerily smart](https://lcamtuf.blogspot.com/2014/11/pulling-jpegs-out-of-thin-air.html), but has been most heavily used in the realm of security, often with a focus on [parsing untrusted inputs](https://google.github.io/oss-fuzz/faq/#what-kind-of-projects-are-you-accepting).

Security is critical, but:

 1. Eventually, the bigger success for fuzzing might be **finding correctness & stability problems** in a broader set of code bases, and beyond today's more common security focus.
 2. There is also a large opportunity to **make fuzzing easier to pick up** by a broader community.

This project aims to pitch in on those two fronts.

If enough people work to make the fuzzing ecosystem accessible enough, "coffee break fuzzing" might eventually become as common as unit tests. And of course, increased adoption of fuzzing helps security as well. :blush:

## Quick Start: Install & Automatically Create Fuzz Targets

For now, the recommendation is to use Go 1.17 for almost all the commands here, and then use [gotip](https://pkg.go.dev/golang.org/dl/gotip) as shown when it is time to kick off the fuzzing.

Starting from an empty directory, create a module and install the dev version of Go 1.18 via gotip:
```
$ go mod init example
$ go install golang.org/dl/gotip@latest
$ gotip download
```

Download and install the go-fuzz-fill-utils binary from source, as well as add its fuzzer to our go.mod:
```
$ go install github.com/infosecual/go-fuzz-fill-utils/cmd/go-fuzz-fill-utils@latest
$ go get github.com/infosecual/go-fuzz-fill-utils/fuzzer
```

Use go-fuzz-fill-utils to automatically create a set of fuzz targets — in this case for the encoding/ascii85 package from the Go standard library:
```
$ go-fuzz-fill-utils encoding/ascii85
go-fuzz-fill-utils: created autofuzz_test.go
```

That's it — now we can start fuzzing!
```
$ gotip test -fuzz=Fuzz_Encode
```

Within a few seconds, you should get a crash:
```
[...]
fuzz: minimizing 56-byte failing input file
fuzz: elapsed: 0s, minimizing
--- FAIL: Fuzz_Encode (0.06s)
```

Without any manual work, you just found a bug in the standard library. (It's a very minor bug though — probably at the level of "perhaps the doc could be more explicit about an expected panic").

That's enough for you to get started on your own, but let's also briefly look at a more interesting example.

## What Do Some Fuzzing Targets Look Like?

When we ran go-fuzz-fill-utils above against `encoding/ascii85`, it automatically created a set of [six](https://github.com/infosecual/go-fuzz-fill-utils/blob/main/examples/outputs/stdlib-encoding-ascii85/autofuzz_test.go) independent fuzzing targets.

A different example is `go-fuzz-fill-utils github.com/google/syzkaller/pkg/report`, which generates [ten](https://github.com/infosecual/go-fuzz-fill-utils/blob/main/examples/outputs/syzkaller-report/autofuzz_test.go) independent fuzzing targets.

Let's look at one of them more closely — the code targeting the [Symbolize](https://pkg.go.dev/github.com/google/syzkaller@v0.0.0-20220105142835-6acc789ad3f6/pkg/report#Reporter.Symbolize) method on the [Reporter](https://pkg.go.dev/github.com/google/syzkaller@v0.0.0-20220105142835-6acc789ad3f6/pkg/report#Reporter) type, along with some added explanatory comments:

```go
// Fuzz_Reporter_Symbolize has the standard signature for Go 1.18 fuzzing.
func Fuzz_Reporter_Symbolize(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
		// go-fuzz-fill-utils declared variables for two structs.
		var cfg *mgrconfig.Config
		var rep *report.Report

		// Structs are not natively supported by Go 1.18, so go-fuzz-fill-utils created an auxiliary fuzzer
		// that fills in the cfg & rep structs with arbitrary data via fz.Fill.
		fz := fuzzer.NewFuzzer(data)
		fz.Fill(&cfg, &rep)

		// A crash on a nil parameter is a bit boring, so by default go-fuzz-fill-utils
		// skips over nil parameters. It is easy to delete these lines if you want.
		if cfg == nil || rep == nil {
			return
		}

		// go-fuzz-fill-utils inserted a call to the NewReporter constructor, which is helpful
		// because the constructor has non-trivial logic and sets unexported members.
		// This itself might panic, or set up an "interesting" reporter for use below.
		reporter, err := report.NewReporter(cfg)

		// Usually it makes sense to stop if a constructor returns a non-nil error.
		if err != nil {
			return
		}

		// Finally, call the targeted Symbolize method using everything we set up above.
		// If it panics, bingo!
		reporter.Symbolize(rep)
	})
}
```

That's close to what a first-cut handwritten fuzz function might look like if we were to target a single method, but it is shorter (because `fz.Fill` populates on our behalf the 50 or so public fields of the `cfg` and `rep` structs) and we did no manual work to write it. :grinning: We might consider using it as is, or extending it from there (e.g., perhaps return immediately if meeting a condition that is documented to panic).

But what if we wanted to target multiple methods at once? That's where `-chain` comes in, which we'll look at next.

## Example: Easily Finding a Data Race

Again starting from an empty directory, we'll set up a module, and add the go-fuzz-fill-utils fuzzer to go.mod:
```
$ go mod init temp
$ go get github.com/infosecual/go-fuzz-fill-utils/fuzzer
```

Next, we automatically create a new fuzz target. This time:
 * We ask go-fuzz-fill-utils to "chain" a set of methods together in a calling sequence controlled by go-fuzz-fill-utils.Fuzzer (via the `-chain` argument).
 * We also tell go-fuzz-fill-utils that it should in theory be safe to do parallel execution of those methods across multiple goroutines (via the `-parallel` argument).

```
$ go-fuzz-fill-utils -chain -parallel github.com/infosecual/go-fuzz-fill-utils/examples/inputs/race
go-fuzz-fill-utils: created autofuzzchain_test.go
```

That's it! Let's get fuzzing. 

This time, we also enable the race detector as we fuzz:
```
$ gotip test -fuzz=. -race
```

This is a harder challenge than our first example, but within several minutes or so, you should get a data race detected:
```
--- FAIL: Fuzz_NewMySafeMap (4.26s)
    --- FAIL: Fuzz_NewMySafeMap (0.01s)
        testing.go:1282: race detected during execution of test
```
 
If we want to see what exact calls triggered this, along with their input arguments, we can ask 'go test' to re-run the discovered failing input
after first setting a debug flag to ask go-fuzz-fill-utils to reconstruct the equivalent Go code that reproduces the discovered failure. (Your failing
example will have a different filename and show a different pattern of calls).

```
$ export FZDEBUG=repro=1                   # On Windows:  set FZDEBUG=repro=1
$ gotip test -run=./9800b52 -race
```

This will output a snippet of valid Go code that was "discovered" at execution time by fuzzing:
```go
        // EXECUTION SEQUENCE (loop count: 1, spin: true)

        Fuzz_MySafeMap_Store(
                [16]uint8{152,152,152,152,152,152,152,152,152,152,152,152,152,152,152,152},
                &raceexample.Request{Answer:42,},
        )

        var wg sync.WaitGroup
        wg.Add(2)

        // Execute next steps in parallel.
        go func() {
                defer wg.Done()
                Fuzz_MySafeMap_Load(
                        [16]uint8{152,152,152,152,152,152,152,152,152,152,152,152,152,152,152,152},
                )
        }()
        go func() {
                defer wg.Done()
                Fuzz_MySafeMap_Load(
                        [16]uint8{152,152,152,152,152,152,152,152,152,152,152,152,152,152,152,152},
                )
        }()
        wg.Wait()

        // Resume sequential execution.
        Fuzz_MySafeMap_Load(
                [16]uint8{0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0},
        )
```

#### What just happened?

Rewinding slightly, if you look at the code you just automatically generated in [autofuzzchain_test.go](https://github.com/infosecual/go-fuzz-fill-utils/blob/main/examples/outputs/race/autofuzzchain_test.go), you can see go-fuzz-fill-utils emitted code that calls a constructor to create a target struct:

```go
	f.Fuzz(func(t *testing.T, data []byte) {
		target := raceexample.NewMySafeMap()
```

Then there are a set of possible "steps" listed that each invoke different methods on the target:

```go
		steps := []fuzzer.Step{
			{
				Name: "Fuzz_MySafeMap_Store",
				Func: func(key [16]byte, req *raceexample.Request) {
					target.Store(key, req)
				},
			},
			{
				Name: "Fuzz_MySafeMap_Load",
				Func: func(key [16]byte) *raceexample.Request {
					return target.Load(key)
				},
			},
```

The steps are a list of closures that each manipulate the target struct in some way. In this case, a `Store` operation that stores a request struct using a large 16-byte key, and a `Load` operation that fetches a request struct based on a key.

Finally, the most important line in that file is:

```go
    // Execute a specific chain of steps, with the count, sequence and arguments controlled by fz.Chain
    fz.Chain(steps, fuzzer.ChainParallel)
```

At execution time, fz.Chain does not just run the steps in the order listed in the code. Rather, it "chains" them together in novel ways with different interesting arguments. For example, the code might list the steps A, B, C, D, but at execution time, fz.Chain might call first call C with some interesting arguments, then take one of C's return value and pass it to B if one of B's inputs has a matching type, then call A twice, then restart with a completely different sequence and arguments. In other words, the steps describe a universe of possibilities, and at execution time fz.Chain guides the underlying fuzzing engine towards interesting calling patterns & arguments within that universe, where the coverage guidance from method Foo can help progress method Bar and vice versa under the full generality a fuzzer can produce.

#### A picture is worth a thousand words

Here is a simplified picture showing the relationship we just described above:
```
      CODE GEN     |                 FUZZING EXECUTION
  ---------------------------------------------------------------------------
                   |
   List of Steps   |    Execution Chain 1     ...     Execution Chain 173,321
                   |
    ┌──────────┐   |       New input                     Interesting input
    │ Method A │   |           |                               |
    └──────────┘   |           v                               v
    ┌──────────┐   |     ┌──────────┐                    ┌──────────┐
    │ Method B │   |     │ Method C │--+ C's          +--│ Method D │
    └──────────┘   |     └──────────┘  │ return       │  └──────────┘
    ┌──────────┐   |                   │ value        │
    │ Method C │   |   +---------------+              │  Run A and C in parallel
    └──────────┘   |   │                              │  Reuse D's input arg for A & C
    ┌──────────┐   |   │  ┌──────────┐                │
    │ Method D │   |   +->│ Method B │                +-----------------+
    └──────────┘   |      └──────────┘                │                 │
                   |                                  │  ┌──────────┐   │  ┌──────────┐
                   |                                  +->│ Method A │   +->│ Method C │
                   |                                     └──────────┘      └──────────┘
```

#### What did the -parallel flag do?

Because the call to fz.Chain includes the `fuzzer.ChainParallel` option due to the `-parallel` flag, fz.Chain simultaneously explores at execution time which operations run in parallel vs. sequentially, the timing of parallel calls, and so on, in an effort to trigger different types of concurrency bugs. Because concurrent operations can be nondeterministic and nondeterministism is a challenge for a coverage-guided fuzzing engine, fz.Chain also attempts to balance enough deterministic behavior to help the underlying fuzzing engine find interesting inputs.

At execution time, the chain is represented as an in-memory directed acyclic graph of different execution operations, with appropriate waits if needed based on what types match and are re-used between inputs and outputs. The `FZDEBUG=repro=1` option causes that runtime representation to be emitted as an equivalent Go code reproducer.

If you look back at the reproducer included above, you can see it "discovered" a particular pattern of parallel vs. sequential calls (as shown by the `go` keywords, `wg.Wait()` and so on) that triggered this data race. 

For this bug, it usually takes hundreds of thousands of coverage-guided runtime variations of call patterns and inputs before hitting this data race. The reproducer emitted was a code-based rendition of the first runtime variation to trigger the race detector.

#### Would the race detector find that bug with a trivial test?

Just running a regular test under the race detector might not catch that bug, including because the race detector [only finds data races that happen at execution time](https://go.dev/blog/race-detector), which means a diversity of code paths and input data are important for the race detector to do its job.
fz.Chain helps supply those code paths and data.

For that bug to be observable by the race detector, [it](https://github.com/infosecual/go-fuzz-fill-utils/blob/main/examples/inputs/race/race.go#L34-L36) requires:

  1. A Store must complete, then be followed by two Loads, and all three must use the same key.
  2. The Store must have certain payload data (`Answer: 42`).
  3. The two Loads must happen concurrently, without another Store first altering the stored payload.

In the snippet of code shown in the reproducer above, the `42` must be `42` to trigger the data race. On the other hand, the exact value of `152,152,152,...` in the key doesn't matter, but what does matter is that the same key value must be used across this sequence of three calls to trigger the bug.

fz.Chain has logic to sometimes re-use input arguments of the same type across different calls (e.g., to make it easier to have a meaningful `Get(key)` following a `Put(key, ...)`), as well logic to feed outputs of one step as the input to a subsequent step, which is helpful in other cases.

## Example: Finding a Real Concurrency Bug in Real Code

The prior example was a small but challenging example that takes a few minutes of fuzzing to find, but you can see an example of a deadlock found in real code [here](https://github.com/infosecual/go-fuzz-fill-utils/blob/main/examples/outputs/race-xsync-map-repro/standalone_repro1_test.go) from the xsync.Map from github.com/puzpuzpuz/xsync.

In this case, it usually takes fz.Chain several million coverage-guided variations over a few hours of executing the generated 
 [autogenchain_test.go](https://github.com/infosecual/go-fuzz-fill-utils/blob/main/examples/outputs/race-xsync-map/autofuzzchain_test.go) before it finds the deadlock.

Interestingly, the deadlock then typically only reproduces about 1 out of 1,000 attempts for a particular discovered problematic calling pattern and arguments.

Fortunately, once a problem is reported, we can paste the output of `FZDEBUG=repro=1` into a [standalone_repro_test.go](https://github.com/infosecual/go-fuzz-fill-utils/blob/main/examples/outputs/race-xsync-map-repro/standalone_repro1_test.go) file and use the handy `-count` argument in a normal `go test -count=10000` invocation, and now we can reproduce the deadlock cleanly on demand. At that point, the reproducer is completely standalone and does not rely on go-fuzz-fill-utils any longer.

## go-fuzz-fill-utils status

* go-fuzz-fill-utils is still a work in progress, but hopefully will soon be approaching beta quality. 
* Emitting reproducers for a chain is currently best effort, but the intent is to improve to creating a complete standalone reproducer.
* Corpus encoding in particular will change in the near term.
* Roughly around the time of Go 1.18 graduating from Beta, the current intent is that go-fuzz-fill-utils will reach a v1.0.0 status.
    * By v1.0.0, go-fuzz-fill-utils will have a stable corpus encoding, or an equivalent (such as perhaps the ability to programmatically set an encoding version number to keep using a corpus that is an older go-fuzz-fill-utils encoding).

## What next?

Any and all feedback is welcome! :grinning:

Please feel free to open a new issue here, or to contact the author on Twitter ([@infosecual1](https://twitter.com/infosecual)).

The [Roadmap](https://github.com/infosecual/go-fuzz-fill-utils/wiki/Roadmap) wiki page is a reasonable starting place.

If you made it this far — thanks!
