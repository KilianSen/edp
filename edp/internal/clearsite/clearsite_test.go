package clearsite

import "testing"

func TestHeaderFormatsAndFiltersDirectives(t *testing.T) {
	got := Header("cookies, storage, bogus, cache")
	want := `"cookies", "storage", "cache"` // bogus dropped, order preserved
	if got != want {
		t.Errorf("Header = %q, want %q", got, want)
	}
	if Header("") != "" || Header("nope") != "" {
		t.Error("expected empty header for empty/invalid input")
	}
}

func TestMarkTakeIsOneShot(t *testing.T) {
	f := New()
	if f.Take(1) != "" {
		t.Error("expected no pending mark initially")
	}
	f.Mark(1, "cookies,cache")
	first := f.Take(1)
	if first != `"cookies", "cache"` {
		t.Errorf("first take = %q", first)
	}
	if second := f.Take(1); second != "" {
		t.Errorf("expected mark consumed once, got %q on second take", second)
	}
}

func TestMarkEmptyIsNoop(t *testing.T) {
	f := New()
	f.Mark(1, "")
	f.Mark(1, "   ")
	if f.Take(1) != "" {
		t.Error("empty directives should not arm a clear")
	}
}

func TestMarksArePerEnv(t *testing.T) {
	f := New()
	f.Mark(1, "cookies")
	f.Mark(2, "cache")
	if f.Take(2) != `"cache"` {
		t.Error("env 2 mark wrong")
	}
	if f.Take(1) != `"cookies"` {
		t.Error("env 1 mark should be independent of env 2")
	}
}
