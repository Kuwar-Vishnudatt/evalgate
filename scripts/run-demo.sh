#!/usr/bin/env bash
# Runs the bundled example target in its "good" and "degraded" forms
# through the full eval-and-gate pipeline, demonstrating that the gate
# correctly passes the good one and blocks the degraded one — no API key
# needed, since the demo uses the deterministic Fake judge.
set -euo pipefail
cd "$(dirname "$0")/.."

OUT=$(mktemp -d -t evalgate-demo.XXXXXX)
trap 'rm -rf "$OUT"' EXIT

echo "--- running the good target (this becomes the baseline) ---"
./bin/evalgate run -cases testdata/golden.json -target good -out "$OUT/good.json"

echo
echo "--- running the degraded target (simulates a regression: worse retrieval + a fabricated claim) ---"
./bin/evalgate run -cases testdata/golden.json -target degraded -out "$OUT/degraded.json"

echo
echo "--- gate: good target against its own thresholds (expected: PASS) ---"
./bin/evalgate check -candidate "$OUT/good.json" -config .evalgate.yaml || true

echo
echo "--- gate: degraded target against the good baseline (expected: BLOCKED) ---"
./bin/evalgate check -candidate "$OUT/degraded.json" -baseline "$OUT/good.json" -config .evalgate.yaml || true
