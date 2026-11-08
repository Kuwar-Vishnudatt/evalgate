# Architecture

## What problem this solves

"Evals" and "agents" are the two terms the 2026 AI Engineer hiring
research found in the majority of postings — and the actual
differentiator inside "evals" isn't running an eval once, it's wiring eval
discipline into CI so a regression in retrieval quality or answer
faithfulness blocks a PR automatically, the same way a failing unit test
would, instead of being caught (or not) by someone eyeballing a demo.
evalgate is that: a golden-test-set + RAGAS-style metric harness
*productized as a reusable CI gate* — a config file, a CLI with proper
exit codes, and a real GitHub Actions workflow — not a one-off notebook
script that computes some scores and prints them.

## Why this is a different project from evalbench (this portfolio's SDE project)

Both projects score LLM outputs against a golden set, which could look
redundant at a glance. They solve different problems:

- **evalbench** (SDE) is about running potentially thousands of judge
  calls *reliably and statistically honestly*: bounded concurrency,
  content-addressable caching, bootstrap confidence intervals over a
  finite golden set, paired comparison so per-case difficulty variance
  doesn't masquerade as a real regression signal. Its scope is the
  distributed-systems-and-statistics problem underneath evaluation.
- **evalgate** (this project) is about the *developer experience and CI
  integration* layer: specific RAGAS-style metrics (faithfulness, answer
  relevancy, context precision, context recall — not evalbench's single
  generic scalar score), a declarative thresholds-and-regression-tolerance
  config, and a CLI with the exact two commands (`run`, `check`) and exit
  codes a GitHub Actions job actually calls. There's no statistical
  confidence-interval machinery here at all — evalgate assumes you already
  have (or don't need) that rigor and asks a narrower, CI-shaped question:
  "did this PR's version of the target regress, yes or no, exit code
  accordingly."

Put differently: evalbench answers "can I trust this number," evalgate
answers "should this PR merge."

## The four metrics, and why two are judge-based and two are algorithmic

RAGAS's actual metric set splits cleanly into two kinds of question, and
`internal/judge` and `internal/metrics` are separated along exactly that
line:

- **Faithfulness** and **answer relevancy** are semantic judgments — "does
  this answer only claim what the context supports," "does this answer
  address the question" — that fundamentally require language
  understanding. These live in `internal/judge` behind a `Judge`
  interface, with a deterministic `Fake` (word-overlap heuristics, no API
  key, used by every test and the bundled demo) and a real `HTTPJudge`
  that prompts an OpenAI-compatible endpoint using RAGAS's actual
  strategy — decompose the answer into atomic claims and ask YES/NO per
  claim against context, rather than one vague "rate this 1-10" prompt
  that would conflate fluency with faithfulness.
- **Context precision** and **context recall** are set/rank comparisons
  against a golden case's `RelevantContextIDs` ground truth — genuinely
  algorithmic once that ground truth exists, no judge call needed. These
  live in `internal/metrics`.

## Context precision is rank-weighted, not naive precision

`internal/metrics.ContextPrecision` is RAGAS's actual formula: for every
retrieved position that's relevant, compute precision at that rank
(relevant-so-far / rank), then average over however many relevant
positions there were. This specifically rewards relevant context appearing
**early** in the retrieved list — the same single relevant hit scores
higher at rank 1 than at rank 3, which `TestContextPrecisionRewardsEarlyRelevantOverLate`
asserts directly. A naive "relevant retrieved / total retrieved" precision
would score both cases identically, discarding the ranking signal that's
the entire point of a retriever doing a good job, not just a complete one.

## A real heuristic bug this project found and fixed, honestly

The first version of `judge.Fake.ScoreAnswerRelevancy` used symmetric
**Jaccard similarity** between question and answer word sets. Running the
bundled good-vs-degraded example end to end surfaced the problem
immediately: a short question ("How long is the warranty?") against a
long, fully-relevant, fully-supported answer scored a LOW relevancy
(~0.06) — not because the answer was off-topic, but because Jaccard's
union-based denominator penalizes an answer merely for being more
detailed than the question, which is completely normal in real Q&A and
has nothing to do with relevance.

Fixed by switching to **containment** — the fraction of the *question's*
words found in the answer, a recall-style measure against the smaller,
fixed-size set — which doesn't get diluted by an answer legitimately
containing more content than the question. A stop-word filter was added
at the same time (`internal/judge/fake.go`'s `stopWords`), since without
it, "the"/"is"/"a" dominate a short question's word set and drown out the
one or two words that actually carry meaning. Both the original failure
and the fix are preserved as a regression test,
`TestAnswerRelevancyNotPenalizedForBeingMoreDetailedThanQuestion`, named
for the exact failure mode it exists to catch.

## The capstone test: the gate actually blocks a real regression

`internal/example` ships a `GoodTarget` and `DegradedTarget` answering the
same tiny product-FAQ knowledge base — the degraded variant retrieves only
the single best-matching document (hurting recall when more context would
help) and always appends one fabricated, unsupported sentence (hurting
faithfulness), simulating a realistic two-dimensional regression: a
retrieval config change plus a prompt that over-generates, together.
`TestCIGateBlocksTheDegradedTargetButPassesTheGoodOne` wires the real
runner, the real Fake judge, the real metrics, and the real gate decision
together and asserts the opposite verdict for each target — not just that
their scores differ, but that a CI job using this exact gate logic would
pass one and block the other. `.github/workflows/ci.yml` runs this same
scenario for real in GitHub Actions on every push, as a second,
independent confirmation beyond the Go test suite.

## Testing strategy

- `internal/judge`: faithfulness correctly distinguishes a well-supported
  answer from a hallucinated one, and scores a claim-by-claim mix
  correctly (not as one undifferentiated blob); answer relevancy's
  containment-vs-Jaccard regression test described above.
- `internal/metrics`: every edge case of the rank-weighted precision
  formula (known hand-computed value, early-vs-late ranking, empty
  retrieved/relevant sets) and recall's order-independence.
- `internal/runner`: per-case scoring, and that a target error is recorded
  separately from (not averaged into) the aggregate — a crashing case
  must not silently drag quality scores toward zero.
- `internal/gate`: absolute-threshold failures, regression-beyond-
  tolerance failures even when still above threshold, the exact boundary
  condition (a drop equal to the tolerance must NOT block — only strictly
  exceeding it), and that an improvement never blocks regardless of
  thresholds.
- `internal/example`: the good-vs-degraded capstone described above.

## What's deliberately out of scope (v1)

- **Multi-rubric or custom metrics beyond the four RAGAS ones
  implemented.** The architecture (a `Judge` interface plus an algorithmic
  metrics package) generalizes to more metrics without changing the gate
  or runner; adding them wasn't the engineering question this project set
  out to answer.
- **A hosted baseline store.** `check -baseline <path>` takes a JSON file
  the CI workflow is expected to produce in a prior step (e.g., checked
  into the repo, or an artifact from main's last successful run) rather
  than evalgate itself owning baseline storage/versioning.
- **Posting results as a PR comment.** The gate's CI contract is its exit
  code and stdout report; wiring that into a GitHub PR comment is a
  thin, repo-specific follow-up, not a new engineering problem.
