package scraper_test

import (
	"testing"
	"time"

	"github.com/elojah/scraper"
)

var t0 = time.Date(2026, 10, 27, 23, 0, 0, 0, time.UTC)

// series is 3 quarter-hours from t0, then a gap of one, then an hour.
func series() scraper.Series {
	q := 15 * time.Minute

	return scraper.Series{Unit: "EUR/MWh", Points: []scraper.Point{
		{Start: t0, Duration: q, Value: 1},
		{Start: t0.Add(q), Duration: q, Value: 2},
		{Start: t0.Add(2 * q), Duration: q, Value: 3},
		{Start: t0.Add(4 * q), Duration: time.Hour, Value: 4},
	}}
}

func TestAt(t *testing.T) {
	t.Parallel()

	s := series()
	for _, c := range []struct {
		name string
		at   time.Time
		want float64
		ok   bool
	}{
		{"before", t0.Add(-time.Nanosecond), 0, false},
		{"first start", t0, 1, true},
		{"first end", t0.Add(15*time.Minute - time.Nanosecond), 1, true},
		{"second start", t0.Add(15 * time.Minute), 2, true},
		{"gap", t0.Add(50 * time.Minute), 0, false},
		{"hour", t0.Add(90 * time.Minute), 4, true},
		{"end", s.End(), 0, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			p, ok := s.At(c.at)
			if ok != c.ok || p.Value != c.want {
				t.Fatalf("At(%s) = %v, %t, want %v, %t", c.at, p.Value, ok, c.want, c.ok)
			}
		})
	}
}

func TestAt_Empty(t *testing.T) {
	t.Parallel()

	if _, ok := (scraper.Series{}).At(t0); ok {
		t.Fatal("At on an empty series found a point")
	}
}

func TestBetween(t *testing.T) {
	t.Parallel()

	s := series()
	for _, c := range []struct {
		name     string
		from, to time.Time
		want     []float64
	}{
		{"all", t0, s.End(), []float64{1, 2, 3, 4}},
		{"wider", t0.Add(-time.Hour), s.End().Add(time.Hour), []float64{1, 2, 3, 4}},
		{"inside a point", t0.Add(time.Minute), t0.Add(31 * time.Minute), []float64{2, 3}},
		{"gap", t0.Add(45 * time.Minute), t0.Add(time.Hour), nil},
		{"to excluded", t0, t0.Add(15 * time.Minute), []float64{1}},
		{"reversed", s.End(), t0, nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			got := s.Between(c.from, c.to)
			if got.Unit != s.Unit {
				t.Fatalf("Unit %q, want %q", got.Unit, s.Unit)
			}
			if len(got.Points) != len(c.want) {
				t.Fatalf("%d points, want %d", len(got.Points), len(c.want))
			}
			for i, p := range got.Points {
				if p.Value != c.want[i] {
					t.Fatalf("point %d is %v, want %v", i, p.Value, c.want[i])
				}
			}
		})
	}
}

// TestBetween_Append checks that appending to what Between returns leaves the series as it was.
func TestBetween_Append(t *testing.T) {
	t.Parallel()

	s := series()
	got := s.Between(t0, t0.Add(15*time.Minute))
	_ = append(got.Points, scraper.Point{Value: 42})

	if s.Points[1].Value != 2 {
		t.Fatalf("point 1 is %v after an append to Between's, want 2", s.Points[1].Value)
	}
}

func TestEnd(t *testing.T) {
	t.Parallel()

	if got, want := series().End(), t0.Add(2*time.Hour); !got.Equal(want) {
		t.Fatalf("End %s, want %s", got, want)
	}
	if got := (scraper.Series{}).End(); !got.IsZero() {
		t.Fatalf("End of an empty series %s, want zero", got)
	}
}
