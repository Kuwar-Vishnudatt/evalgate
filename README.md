# evalgate

A RAGAS-style eval harness productized as a reusable CI gate: a golden
test set, four real metrics (faithfulness, answer relevancy, context
precision, context recall — two judge-based, two algorithmic against
ground-truth context IDs), a declarative threshold-and-regression-
tolerance config, and a CLI with the two commands and exit codes a CI job
actually needs. Not a one-off eval script — a tool meant to be dropped
into any RAG application's repo.

```
golden.json ──▶ target.Generate(question) ──▶ (answer, retrieved contexts)
                                                        │
                              judge.Judge: faithfulness, answer relevancy
                              metrics:     context precision, context recall
                                                        │
                                              per-case + aggregate scores
                                                        │
                         gate.Evaluate(candidate, baseline, thresholds) ──▶ pass / BLOCKED
```

## Why this project

The 2026 AI Engineer hiring research found "evals" in the majority of
postings — and the differentiator inside that is CI integration, not
running an eval once and eyeballing the output. See
[docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for the full reasoning,
including why this is a deliberately different project from this
portfolio's `evalbench` (that one solves the statistics-and-concurrency
problem underneath evaluation; this one solves the CI/developer-experience
problem on top of it), the rank-weighted context precision formula, and a
real heuristic bug (Jaccard penalizing a detailed answer for being longer
than its question) found by running the bundled example end to end and
fixed with a named regression test.

## Quickstart

```bash
make build
make test        # 40+ tests, no external services, no API key
make demo         # good vs degraded example target through the full pipeline
```

```bash
./bin/evalgate run -cases testdata/golden.json -target good -out baseline.json
./bin/evalgate run -cases testdata/golden.json -target degraded -out candidate.json
./bin/evalgate check -candidate candidate.json -baseline baseline.json -config .evalgate.yaml
# exit 1: faithfulness regressed below threshold — this is the PR check that blocks a merge
```

`.evalgate.yaml` (committed once per repo as policy):
```yaml
min_faithfulness: 0.75
min_answer_relevancy: 0.1
min_context_precision: 0.7
min_context_recall: 0.7
max_regression: 0.1   # max allowed drop vs baseline, even if still above threshold
```

## Using this as a real CI gate

`.github/workflows/ci.yml` in this repo runs the full good-vs-degraded
scenario for real in GitHub Actions on every push — proving the gate
actually blocks a real regression in CI, not just in a unit test (the
`eval-gate-self-check` job). Wiring in your own RAG application: implement
`target.Target`'s one method (`Generate(ctx, question) (Response, error)`)
and call `internal/runner.Run` directly from Go code (or extend the CLI's
`run` command to construct your target instead of the bundled example) —
everything downstream (judge, metrics, gate) is unchanged.

## What's actually tested, concretely

- **Faithfulness** correctly distinguishes a well-supported answer from a
  hallucinated one, scoring per-claim rather than as one blob.
- **Context precision** is proven rank-weighted: the same single relevant
  hit scores higher at rank 1 than buried at rank 3 — a naive
  "relevant/total" precision would treat these identically.
- **A real regression found and fixed during this project**: the Fake
  judge's original answer-relevancy heuristic (Jaccard similarity)
  penalized a fully relevant, fully-supported answer purely for being
  longer than its question — found by running the bundled example
  end-to-end, fixed by switching to a containment-style measure, and kept
  honest with a named regression test.
- **The capstone**: the real gate decision wired to the real runner,
  judge, and metrics — proving it passes a "good" target and blocks a
  "degraded" one that retrieves worse and fabricates a claim, both in Go
  tests and for real in the bundled GitHub Actions workflow.

## Project layout

```
internal/golden/      golden test set format + loading
internal/target/       Target interface (what's under evaluation)
internal/judge/         Faithfulness + answer relevancy (Fake + real HTTP judge)
internal/metrics/       Context precision (rank-weighted) + recall, algorithmic
internal/runner/        orchestrates target+judge+metrics into per-case/aggregate scores
internal/gate/           threshold + regression-tolerance CI decision, YAML config
internal/example/        bundled good/degraded example target + FAQ knowledge base
cmd/evalgate/             CLI: run, check
.github/workflows/ci.yml  real CI: tests + a self-check that the gate blocks a real regression
docs/ARCHITECTURE.md      design rationale
```

## What's deliberately out of scope (v1)

See docs/ARCHITECTURE.md — no custom/multi-rubric metrics beyond the four
RAGAS ones here (the architecture generalizes without changes), no hosted
baseline store (a baseline is just a JSON file your CI workflow produces),
no automatic PR-comment posting.
