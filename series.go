package scraper

import (
	"sort"
	"time"
)

// Point is a value over [Start, Start+Duration).
type Point struct {
	Start    time.Time
	Duration time.Duration
	Value    float64
}

// End is when the point's value stops.
func (p Point) End() time.Time {
	return p.Start.Add(p.Duration)
}

// Series is values in Unit, such as "EUR/MWh" or "MW", over points sorted by Start, none overlapping another. Points
// may leave gaps between them.
type Series struct {
	Unit   string
	Points []Point
}

// At returns the point whose span holds t, false if t falls in a gap or out of the series.
func (s Series) At(t time.Time) (Point, bool) {
	i := sort.Search(len(s.Points), func(i int) bool { return s.Points[i].End().After(t) })
	if i == len(s.Points) || s.Points[i].Start.After(t) {
		return Point{}, false
	}

	return s.Points[i], true
}

// Between returns the points starting in [from, to). They share s's array.
func (s Series) Between(from, to time.Time) Series {
	i := sort.Search(len(s.Points), func(i int) bool { return !s.Points[i].Start.Before(from) })
	j := max(i, sort.Search(len(s.Points), func(j int) bool { return !s.Points[j].Start.Before(to) }))

	return Series{Unit: s.Unit, Points: s.Points[i:j:j]}
}

// End is when the last point stops, the zero time if s has none.
func (s Series) End() time.Time {
	if len(s.Points) == 0 {
		return time.Time{}
	}

	return s.Points[len(s.Points)-1].End()
}
