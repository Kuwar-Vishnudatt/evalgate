package metrics

import "testing"

func approx(a, b float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d < 1e-9
}

func TestContextPrecisionAllRelevantAtTopIsOne(t *testing.T) {
	p := ContextPrecision([]string{"a", "b", "c"}, []string{"a", "b", "c"})
	if !approx(p, 1.0) {
		t.Fatalf("expected 1.0 when every retrieved item is relevant, got %v", p)
	}
}

func TestContextPrecisionNoneRelevantIsZero(t *testing.T) {
	p := ContextPrecision([]string{"x", "y", "z"}, []string{"a", "b"})
	if !approx(p, 0.0) {
		t.Fatalf("expected 0.0 when nothing relevant was retrieved, got %v", p)
	}
}

func TestContextPrecisionRewardsEarlyRelevantOverLate(t *testing.T) {
	// Same single relevant hit, but at position 1 vs position 3 — the
	// rank-weighted metric must score the earlier hit higher.
	early := ContextPrecision([]string{"a", "x", "y"}, []string{"a"})
	late := ContextPrecision([]string{"x", "y", "a"}, []string{"a"})
	if early <= late {
		t.Fatalf("expected an early relevant hit (precision=%v) to score higher than a late one (precision=%v)", early, late)
	}
}

func TestContextPrecisionKnownValue(t *testing.T) {
	// retrieved: [relevant, irrelevant, relevant] against relevant={a,c}
	// at a=position1 (id "a"): precision@1 = 1/1 = 1.0
	// at c=position3 (id "c"): precision@3 = 2/3
	// average over the 2 relevant positions = (1.0 + 2/3) / 2 = 0.8333...
	p := ContextPrecision([]string{"a", "b", "c"}, []string{"a", "c"})
	want := (1.0 + 2.0/3.0) / 2.0
	if !approx(p, want) {
		t.Fatalf("expected %v, got %v", want, p)
	}
}

func TestContextPrecisionEmptyRelevantSetIsVacuouslyOne(t *testing.T) {
	p := ContextPrecision([]string{"a", "b"}, nil)
	if !approx(p, 1.0) {
		t.Fatalf("expected 1.0 for an empty ground-truth relevant set, got %v", p)
	}
}

func TestContextPrecisionEmptyRetrievedListIsZero(t *testing.T) {
	p := ContextPrecision(nil, []string{"a"})
	if !approx(p, 0.0) {
		t.Fatalf("expected 0.0 when nothing at all was retrieved but relevant contexts exist, got %v", p)
	}
}

func TestContextRecallAllFoundIsOne(t *testing.T) {
	r := ContextRecall([]string{"a", "b", "c", "x"}, []string{"a", "b"})
	if !approx(r, 1.0) {
		t.Fatalf("expected 1.0 when all relevant contexts were retrieved, got %v", r)
	}
}

func TestContextRecallPartialFound(t *testing.T) {
	r := ContextRecall([]string{"a", "x"}, []string{"a", "b"})
	if !approx(r, 0.5) {
		t.Fatalf("expected 0.5 for 1 of 2 relevant contexts found, got %v", r)
	}
}

func TestContextRecallNoneFoundIsZero(t *testing.T) {
	r := ContextRecall([]string{"x", "y"}, []string{"a", "b"})
	if !approx(r, 0.0) {
		t.Fatalf("expected 0.0 when none of the relevant contexts were retrieved, got %v", r)
	}
}

func TestContextRecallDoesNotCareAboutOrder(t *testing.T) {
	r1 := ContextRecall([]string{"b", "a"}, []string{"a", "b"})
	r2 := ContextRecall([]string{"a", "b"}, []string{"a", "b"})
	if !approx(r1, r2) || !approx(r1, 1.0) {
		t.Fatalf("expected recall to be order-independent, got r1=%v r2=%v", r1, r2)
	}
}

func TestContextRecallEmptyRelevantSetIsVacuouslyOne(t *testing.T) {
	r := ContextRecall([]string{"a"}, nil)
	if !approx(r, 1.0) {
		t.Fatalf("expected 1.0 for an empty ground-truth relevant set, got %v", r)
	}
}

func TestContextRecallEmptyRetrievedListIsZero(t *testing.T) {
	r := ContextRecall(nil, []string{"a"})
	if !approx(r, 0.0) {
		t.Fatalf("expected 0.0 when nothing was retrieved but relevant contexts exist, got %v", r)
	}
}
