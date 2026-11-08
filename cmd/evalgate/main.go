// Command evalgate is the CLI: `run` evaluates a target against a golden
// set and saves scores; `check` compares a candidate run against a
// baseline (optional) and a config's thresholds, exiting non-zero on any
// failure — the actual CI gate.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/Kuwar-Vishnudatt/evalgate/internal/example"
	"github.com/Kuwar-Vishnudatt/evalgate/internal/gate"
	"github.com/Kuwar-Vishnudatt/evalgate/internal/golden"
	"github.com/Kuwar-Vishnudatt/evalgate/internal/judge"
	"github.com/Kuwar-Vishnudatt/evalgate/internal/runner"
	"github.com/Kuwar-Vishnudatt/evalgate/internal/target"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "run":
		runRun(os.Args[2:])
	case "check":
		runCheck(os.Args[2:])
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: evalgate <run|check> [flags]")
}

func runRun(args []string) {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	casesPath := fs.String("cases", "", "path to the golden set JSON file (required)")
	targetName := fs.String("target", "good", `which bundled example target to run: "good" or "degraded" (real usage would wire in your own target.Target and judge.Judge via Go code calling the runner package directly — see README)`)
	out := fs.String("out", "", "path to write the run result JSON (required)")
	fs.Parse(args)

	if *casesPath == "" || *out == "" {
		fmt.Fprintln(os.Stderr, "run: -cases and -out are required")
		os.Exit(2)
	}

	cases, err := golden.Load(*casesPath)
	must(err)

	var t target.Target
	switch *targetName {
	case "good":
		t = example.GoodTarget{Docs: example.FAQDocs()}
	case "degraded":
		t = example.DegradedTarget{Docs: example.FAQDocs()}
	default:
		fmt.Fprintf(os.Stderr, "run: unknown target %q (use \"good\" or \"degraded\")\n", *targetName)
		os.Exit(2)
	}

	res, err := runner.Run(context.Background(), cases, t, judge.NewFake())
	must(err)

	data, err := json.MarshalIndent(res, "", "  ")
	must(err)
	must(os.WriteFile(*out, data, 0o644))

	fmt.Printf("evaluated %d cases (%d errors)\n", res.Aggregate.NumCases, res.Aggregate.NumErrors)
	fmt.Printf("  faithfulness=%.3f answer_relevancy=%.3f context_precision=%.3f context_recall=%.3f\n",
		res.Aggregate.Faithfulness, res.Aggregate.AnswerRelevancy, res.Aggregate.ContextPrecision, res.Aggregate.ContextRecall)
	fmt.Printf("saved to %s\n", *out)
}

func runCheck(args []string) {
	fs := flag.NewFlagSet("check", flag.ExitOnError)
	candidatePath := fs.String("candidate", "", "path to the candidate run's result JSON (required)")
	baselinePath := fs.String("baseline", "", "path to the baseline run's result JSON (optional — omit to check thresholds only)")
	configPath := fs.String("config", ".evalgate.yaml", "path to the thresholds config file")
	fs.Parse(args)

	if *candidatePath == "" {
		fmt.Fprintln(os.Stderr, "check: -candidate is required")
		os.Exit(2)
	}

	th, err := gate.LoadThresholds(*configPath)
	must(err)

	candidate := loadResult(*candidatePath)

	var baselineAgg *runner.Aggregate
	if *baselinePath != "" {
		b := loadResult(*baselinePath)
		baselineAgg = &b.Aggregate
	}

	decision := gate.Evaluate(candidate.Aggregate, baselineAgg, th)
	fmt.Print(gate.Report(decision))

	if decision.Blocked {
		os.Exit(1)
	}
}

func loadResult(path string) runner.Result {
	data, err := os.ReadFile(path)
	must(err)
	var res runner.Result
	must(json.Unmarshal(data, &res))
	return res
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
