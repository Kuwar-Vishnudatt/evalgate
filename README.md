# evalgate

**A CI gate for RAG systems: it answers "should this PR merge," not "is my
model good."**

evalgate is a small Go tool that plugs into a continuous-integration
pipeline and automatically fails a pull request when a retrieval-augmented
generation (RAG) system — an AI app that looks things up in a knowledge
base before answering, as opposed to answering purely from what the model
memorized — gets worse at either finding the right information or
answering truthfully with it. It implements four metrics from **RAGAS**
(Retrieval-Augmented Generation Assessment, a widely used open framework
for scoring exactly this kind of system), wraps them in a config file you
commit to your repo, and gives you a command-line tool with the two
commands and the exit codes a CI job needs to act on the result. It is not
a notebook you run by hand and eyeball — it is meant to sit in
`.github/workflows/` next to your other required checks, the same way a
linter or a test suite does.

## Why this exists, and why it's not the same as its sibling projects

Two things keep showing up in AI engineering job postings right now:
building "agents" and doing "evals" (short for evaluations — systematically
scoring an AI system's outputs instead of just reading a few examples and
deciding it "looks fine"). The gap almost everyone leaves open is that the
evaluation only happens once, by hand, usually right before a demo. Nobody
wires it into the same automated checks that already guard the rest of the
codebase. evalgate closes that specific gap: it is eval *discipline*,
productized as a CI check — a real config file, a real CLI with proper
exit codes, and a real GitHub Actions workflow that proves the gate
actually blocks a bad change — not a one-off script someone runs and
squints at.

This is one of three AI-engineering-flavored projects in a larger
portfolio, and it is worth being explicit about how it differs from its
closest sibling, **evalbench**, because at a glance "we both score LLM
outputs against a golden set" sounds like the same project twice:

- **evalbench** answers *"can I trust this number?"* It is about running
  potentially thousands of judge calls (an LLM call that scores another
  LLM's output) reliably and statistically honestly — bounded concurrency
  so you don't blow through rate limits, caching so repeated runs don't
  re-pay for answers you already scored, bootstrap confidence intervals so
  a score is reported with its uncertainty instead of as a bare number,
  and paired comparison so that two cases being harder or easier than each
  other doesn't get mistaken for a real change in quality. It solves the
  statistics-and-infrastructure problem *underneath* evaluation.
- **evalgate** (this project) answers *"should this PR merge?"* It
  deliberately has none of that statistical machinery. Instead it goes
  deep on one narrower, CI-shaped question: given this specific kind of
  system (RAG), with these four specific, well-known metrics, and a
  policy file that says what "acceptable" and "regressed" mean, did this
  change make things worse — yes or no, with an exit code a GitHub Actions
  job can branch on.

A third sibling, **contractgate**, is a CI gate too, but for *data
contracts* (schema and data-quality rules between pipelines) — it has
nothing to do with LLMs at all. evalgate is the one in the family that is
specifically about shipping LLM-powered applications safely.

## The four metrics, explained simply

evalgate scores every answer on four axes. Two require genuine language
understanding and are computed by a **judge** (anything implementing the
`Judge` interface — either a free, deterministic stand-in used for
testing, or a real LLM call for production use, both described below).
The other two are pure set/rank math against a hand-labeled answer key, no
judge required.

1. **Faithfulness** — does the answer only say things the retrieved
   context actually supports, or does it **hallucinate** (state something
   that sounds plausible but isn't backed by anything it was given)?
   evalgate computes this the way real RAGAS does: it breaks the answer
   into its individual factual claims (roughly, one per sentence) and
   checks each claim against the context separately, rather than asking
   "does this look right?" about the whole answer as one blob.
   *Why this catches a real problem:* a fluent, confident-sounding answer
   can still be making things up, and a single "overall vibe" score would
   let one fabricated sentence hide inside three accurate ones.

2. **Answer relevancy** — does the answer actually address the question
   that was asked, as opposed to being accurate-but-off-topic or padded
   with irrelevant filler?
   *Why this catches a real problem:* a RAG system can be perfectly
   faithful to its context and still fail the user by burying or dodging
   the actual question — faithfulness alone wouldn't catch that.

3. **Context precision** — of the pieces of context the system retrieved,
   were the genuinely relevant ones ranked near the top? This metric is
   **rank-weighted**: it does not just ask "was a relevant result in
   there somewhere," it specifically rewards relevant results for
   showing up *early*. The same single relevant document scores higher
   if it's returned first than if it's buried third behind two irrelevant
   ones, even though a plain "relevant results ÷ total results" count
   would call those two situations identical.
   *Why this catches a real problem:* a retriever that finds the right
   document but ranks it dead last is still a bad retriever in practice —
   most RAG prompts only have room for the top few results, or the model
   simply pays more attention to what it sees first. A retriever that
   puts the right answer first is strictly better than one that buries it
   under five wrong ones, even if both technically "found" it, and this
   metric is the one that notices the difference.

4. **Context recall** — out of everything that *should* have been
   retrieved for a question (known in advance, from a hand-labeled
   "golden" answer key — see below), how much of it actually was? Unlike
   the other three, this is pure set comparison: no judge, no LLM call,
   just checking which of the known-correct context IDs showed up in what
   was retrieved.
   *Why this catches a real problem:* a retriever can rank what it finds
   beautifully and still be missing a crucial document entirely — recall
   is what catches "we never even looked in the right place."

## How it works, end to end

1. **You define a golden test set.** This is a JSON file of cases, each
   with a question, a reference answer, and the IDs of the context
   documents that genuinely answer it (`relevant_context_ids` — the
   ground truth that context precision and recall are measured against).
   This is checked into your repo and reviewed like code, since it *is*
   your definition of "correct" for this system.
2. **Your target AI system answers every question.** "Target" is
   evalgate's name for whatever RAG system is under test — anything that
   implements one small interface: given a question, return an answer
   plus the list of context passages it retrieved to produce it. evalgate
   doesn't care how your retrieval or generation actually works
   internally; it only consumes that (answer, contexts) pair, which is
   the same boundary RAGAS-style evaluation draws in practice.
3. **All four metrics get scored for every case**, then averaged into one
   aggregate score per metric across the whole golden set.
4. **A config file's thresholds and regression rules decide pass or
   fail.** `.evalgate.yaml`, committed once per repo like a style guide,
   sets a minimum acceptable score for each of the four metrics *and* a
   maximum allowed regression against a baseline run (typically main
   branch's last known-good scores). The regression check matters on its
   own, separately from the minimums: a score can be comfortably above
   its floor and still represent a real quality drop worth blocking. For
   example, faithfulness falling from 95% to 80% is a genuine regression
   in how trustworthy the system's answers are, even though 80% still
   clears a 75% minimum — the floor alone would wave that drop straight
   through.
5. **The CLI exits with the right code for CI to act on.** `evalgate
   check` prints a per-metric report and exits `1` if anything is below
   threshold or regressed beyond tolerance, `0` otherwise — which is all
   a GitHub Actions step needs to turn this into a required check that
   blocks a merge.

## Architecture at a glance

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

- **`internal/golden`** — loads and validates the golden test set: each
  case's question, reference answer, and relevant-context-ID ground
  truth.
- **`internal/target`** — the one-method interface (`Generate(ctx,
  question) (Response, error)`) that any RAG system plugs in through.
  evalgate ships two bundled example targets for its own tests and demo,
  not a production retriever.
- **`internal/judge`** — the two semantic metrics. `Fake` is a free,
  deterministic word-overlap judge with no API key, used by every test
  and the bundled demo, specifically so the harness's own logic can be
  proven correct independent of judge quality. `HTTPJudge` is a real judge
  that calls any OpenAI-compatible chat-completions endpoint, using
  RAGAS's actual strategy of decomposing an answer into atomic claims and
  asking yes/no per claim, rather than one vague "rate this 1–10" prompt
  that would blur fluency and faithfulness together.
- **`internal/metrics`** — the two algorithmic metrics, context precision
  (rank-weighted) and context recall, as plain set/rank math against a
  case's ground-truth context IDs — no judge involved.
- **`internal/runner`** — orchestrates target + judge + metrics into
  per-case and aggregate scores. It also keeps a case where the target
  itself crashed separate from the quality averages, instead of silently
  averaging it in as a zero score, so a flaky target doesn't masquerade
  as a genuine quality regression.
- **`internal/gate`** — the CI decision layer: loads `.evalgate.yaml`,
  applies the minimum-threshold and max-regression rules, and produces
  the pass/fail verdict and CI-log report.
- **`internal/example`** — the bundled "good vs degraded" demo target and
  its tiny fictional product-FAQ knowledge base, used by the Go test
  suite and by `make demo`.
- **`cmd/evalgate`** — the CLI: `run` (score a target against the golden
  set, save the result as JSON) and `check` (compare a candidate result
  against a config's thresholds and, optionally, a baseline result).
- **`.github/workflows/ci.yml`** — a real GitHub Actions workflow with two
  jobs: one runs the Go test suite, the other builds the CLI and actually
  runs the good-vs-degraded scenario for real — proving in GitHub Actions
  itself, not just inside a unit test, that the gate passes the good
  target and blocks the degraded one.
- **`docs/ARCHITECTURE.md`** — the fuller design writeup, including the
  rank-weighted precision formula worked through by hand and the
  reasoning behind every "deliberately out of scope" call below.

Wiring in a real RAG application means implementing `target.Target`'s one
method and calling `internal/runner.Run` directly from your own Go code
(or extending the CLI's `run` command to construct your target instead of
the bundled example) — everything downstream (judge, metrics, gate) is
unchanged.

## A real mistake we found and fixed

This is worth telling in full, because it's a good example of exactly the
kind of bug evaluation tooling exists to catch — including in itself.

The first version of the free `Fake` judge's answer-relevancy score used
**Jaccard similarity**: take the set of words in the question, take the
set of words in the answer, and divide the size of their overlap by the
size of their *union*. It's a standard, symmetric way to compare two sets
of words, and it seemed like a reasonable starting point.

Running the bundled "good vs degraded" demo end to end — not just
inspecting the code, actually executing it and looking at the numbers it
produced — immediately turned up something wrong. A short question like
*"How long is the warranty?"* paired with a fully correct, fully
detailed, fully context-supported answer scored an answer relevancy of
roughly **6%**. That's a score so low it would read as "this answer is
almost entirely off-topic," which was obviously false — the answer
directly and correctly addressed the question.

The cause was Jaccard's union-based denominator. A good answer to a
question is, in totally normal question-answering, almost always *longer*
than the question itself — it has to be, since it contains the actual
information the question was asking for. But every extra word the answer
adds that isn't in the question grows the union, which shrinks the
Jaccard score, even though those extra words are exactly what makes the
answer good. The formula was penalizing the answer for being thorough.

The fix was to switch from a symmetric comparison to a **containment**
measure: instead of asking "how much do these two word sets overlap
relative to their combined size," ask "what fraction of the *question's*
words show up somewhere in the answer." That's a fixed-size, one-directional
check — it doesn't get diluted just because the answer happens to say more
than the question did. A stop-word filter ("the," "is," "a," and other
grammatical filler) was added at the same time, since without it, a short
question's word set is dominated by words that carry no real meaning, and
the one or two words that actually matter (like "warranty") get drowned
out.

The original failure and the fix are both preserved permanently as a
named regression test,
`TestAnswerRelevancyNotPenalizedForBeingMoreDetailedThanQuestion` in
`internal/judge/fake_test.go`, specifically so this exact mistake can
never silently come back.

## Try it yourself

```bash
make build
make test   # 40+ tests, entirely offline — no external services, no API key
make demo   # runs the good vs degraded example target through the full pipeline
```

Or run the CLI directly, the same way CI does:

```bash
./bin/evalgate run -cases testdata/golden.json -target good -out baseline.json
./bin/evalgate run -cases testdata/golden.json -target degraded -out candidate.json
./bin/evalgate check -candidate candidate.json -baseline baseline.json -config .evalgate.yaml
# exits 1: faithfulness regressed beyond tolerance — this is the exact check
# that would block the PR in CI
```

`.evalgate.yaml` — the policy file, committed once per repo:

```yaml
min_faithfulness: 0.75
min_answer_relevancy: 0.1
min_context_precision: 0.7
min_context_recall: 0.7
max_regression: 0.1   # max allowed drop vs baseline, even if still above threshold
```

`.github/workflows/ci.yml` runs this exact scenario for real on every
push: it builds the CLI, scores the "good" target as a baseline, scores
the "degraded" target as a candidate, asserts the good target passes the
gate, and asserts the degraded target is blocked by it — so the claim
"this gate actually blocks a regression" is proven in a live CI run, not
just asserted in a README.

## What this deliberately doesn't do (yet)

- **No custom or multi-rubric metrics beyond the four RAGAS metrics
  implemented here.** The `Judge` interface plus the algorithmic metrics
  package generalize to more metrics without changing the gate or runner
  — adding them wasn't the engineering question this project set out to
  answer.
- **No hosted baseline store.** `check -baseline <path>` takes a JSON file
  your own CI workflow is expected to produce (checked into the repo, or
  pulled as an artifact from main's last successful run) — evalgate
  itself doesn't own baseline storage or versioning.
- **No automatic PR-comment posting.** The gate's CI contract is its exit
  code and stdout report; turning that into a posted GitHub PR comment is
  a thin, repo-specific integration, not a new engineering problem this
  project needed to solve.

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for the fuller reasoning
behind all of the above, plus the rank-weighted precision formula worked
through by hand.

## Project layout

```
internal/golden/         golden test set format + loading
internal/target/         Target interface (what's under evaluation)
internal/judge/          faithfulness + answer relevancy (Fake + real HTTP judge)
internal/metrics/        context precision (rank-weighted) + recall, algorithmic
internal/runner/         orchestrates target+judge+metrics into per-case/aggregate scores
internal/gate/           threshold + regression-tolerance CI decision, YAML config
internal/example/        bundled good/degraded example target + FAQ knowledge base
cmd/evalgate/             CLI: run, check
.github/workflows/ci.yml  real CI: tests + a self-check that the gate blocks a real regression
docs/ARCHITECTURE.md      design rationale
```

## License

MIT — see [LICENSE](LICENSE).

Repo: [github.com/Kuwar-Vishnudatt/evalgate](https://github.com/Kuwar-Vishnudatt/evalgate)
