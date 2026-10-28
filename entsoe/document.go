package entsoe

import (
	"cmp"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/elojah/scraper"
)

const (
	// maxPoints bounds the points a document expands into: ENTSO-E answers a year at most, 35 040 quarter-hours a
	// series, 105 120 for A69's three.
	maxPoints = 1 << 18
	// maxResolution bounds a period's resolution: a day's is the longest a minute or hour count gives.
	maxResolution = 24 * time.Hour
)

// Curve types: A01 has every position, A03 leaves out those repeating the previous value.
const (
	curveFixed    = "A01"
	curveVariable = "A03"
)

// Decode reads a document ENTSO-E answers. A Publication_MarketDocument's or a GL_MarketDocument's TimeSeries come
// back merged into a Series per psrType, "" for those without one, each position a Point of its period's resolution.
// An Acknowledgement_MarketDocument comes back as an *AcknowledgementError, anything else as a *DocumentError.
func Decode(r io.Reader) (map[PSRType]scraper.Series, error) {
	d := xml.NewDecoder(r)
	for {
		token, err := d.Token()
		if errors.Is(err, io.EOF) {
			return nil, invalid("no root element")
		}
		if err != nil {
			return nil, invalid("%s", err)
		}

		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}

		switch start.Name.Local {
		case "Publication_MarketDocument", "GL_MarketDocument":
			var doc document
			if err := d.DecodeElement(&doc, &start); err != nil {
				return nil, invalid("%s", err)
			}

			return doc.series()
		case "Acknowledgement_MarketDocument":
			var ack acknowledgement
			if err := d.DecodeElement(&ack, &start); err != nil {
				return nil, invalid("%s", err)
			}

			return nil, ack.err()
		default:
			return nil, invalid("root element %s", start.Name.Local)
		}
	}
}

// acknowledgement is an Acknowledgement_MarketDocument, ENTSO-E's refusal.
type acknowledgement struct {
	Reasons []struct {
		Code string `xml:"code"`
		Text string `xml:"text"`
	} `xml:"Reason"`
}

func (a acknowledgement) err() error {
	if len(a.Reasons) == 0 {
		return &AcknowledgementError{}
	}

	return &AcknowledgementError{Code: a.Reasons[0].Code, Text: strings.TrimSpace(a.Reasons[0].Text)}
}

// document is a Publication_MarketDocument, of prices, or a GL_MarketDocument, of quantities: tags name no namespace,
// so that both versions' match.
type document struct {
	TimeSeries []timeSeries `xml:"TimeSeries"`
}

type timeSeries struct {
	Currency     string   `xml:"currency_Unit.name"`
	PriceUnit    string   `xml:"price_Measure_Unit.name"`
	QuantityUnit string   `xml:"quantity_Measure_Unit.name"`
	CurveType    string   `xml:"curveType"`
	PSRType      PSRType  `xml:"MktPSRType>psrType"`
	Periods      []period `xml:"Period"`
}

type period struct {
	Start      string  `xml:"timeInterval>start"`
	End        string  `xml:"timeInterval>end"`
	Resolution string  `xml:"resolution"`
	Points     []point `xml:"Point"`
}

type point struct {
	Position int    `xml:"position"`
	Price    string `xml:"price.amount"`
	Quantity string `xml:"quantity"`
}

// series expands each TimeSeries' periods and merges them by psrType.
func (doc document) series() (map[PSRType]scraper.Series, error) {
	series := make(map[PSRType]scraper.Series)
	budget := maxPoints
	for i, ts := range doc.TimeSeries {
		var fill bool
		switch ts.CurveType {
		case "", curveFixed:
		case curveVariable:
			fill = true
		default:
			return nil, invalid("TimeSeries %d: curve type %s", i+1, ts.CurveType)
		}

		unit := ts.unit()
		s, ok := series[ts.PSRType]
		if ok && s.Unit != unit {
			return nil, invalid("TimeSeries %d: unit %s, then %s", i+1, s.Unit, unit)
		}
		s.Unit = unit

		for j, p := range ts.Periods {
			points, err := p.expand(fill, &budget)
			if de := (*DocumentError)(nil); errors.As(err, &de) {
				de.Reason = fmt.Sprintf("TimeSeries %d, Period %d: %s", i+1, j+1, de.Reason)
			}
			if err != nil {
				return nil, err
			}
			s.Points = append(s.Points, points...)
		}
		series[ts.PSRType] = s
	}

	for k, s := range series {
		s.Points = merge(s.Points)
		series[k] = s
	}

	return series, nil
}

// unit is "EUR/MWh" for prices, "MW" for quantities: ENTSO-E's MWH and MAW.
func (ts timeSeries) unit() string {
	if ts.Currency == "" && ts.PriceUnit == "" {
		return measure(ts.QuantityUnit)
	}

	return ts.Currency + "/" + measure(ts.PriceUnit)
}

// measure reads a UN/CEFACT unit code, as ENTSO-E writes them.
func measure(code string) string {
	switch code {
	case "MWH":
		return "MWh"
	case "MAW":
		return "MW"
	default:
		return code
	}
}

// expand makes a Point of each position, taken off budget. With fill, a position left out repeats the previous one,
// the last until the period's end.
func (p period) expand(fill bool, budget *int) ([]scraper.Point, error) {
	start, err := parseTime(p.Start)
	if err != nil {
		return nil, err
	}
	end, err := parseTime(p.End)
	if err != nil {
		return nil, err
	}
	resolution, err := parseResolution(p.Resolution)
	if err != nil {
		return nil, err
	}

	span := end.Sub(start)
	if span <= 0 || span%resolution != 0 {
		return nil, invalid("%s to %s is not a whole number of %s", p.Start, p.End, p.Resolution)
	}
	if len(p.Points) == 0 {
		return nil, invalid("no point")
	}

	n := int(span / resolution)
	size := len(p.Points)
	if fill {
		size = n
	}
	if size > *budget {
		return nil, invalid("more than %d points", maxPoints)
	}
	*budget -= size

	points := make([]scraper.Point, 0, size)
	previous := 0
	for i, pt := range p.Points {
		if pt.Position <= previous || pt.Position > n {
			return nil, invalid("position %d, after %d, of %d", pt.Position, previous, n)
		}
		if fill && i == 0 && pt.Position != 1 {
			return nil, invalid("first position %d, not 1", pt.Position)
		}
		value, err := pt.value()
		if err != nil {
			return nil, err
		}

		next := pt.Position + 1
		if fill {
			next = n + 1
			if i+1 < len(p.Points) {
				// Checked by the next point.
				next = min(p.Points[i+1].Position, next)
			}
		}
		for position := pt.Position; position < next; position++ {
			points = append(points, scraper.Point{
				Start:    start.Add(time.Duration(position-1) * resolution),
				Duration: resolution,
				Value:    value,
			})
		}
		previous = pt.Position
	}

	return points, nil
}

func (pt point) value() (float64, error) {
	s := pt.Price
	if s == "" {
		s = pt.Quantity
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, invalid("position %d: value %q", pt.Position, s)
	}

	return v, nil
}

// parseTime reads 2026-10-26T23:00Z, ENTSO-E's, or RFC 3339.
func parseTime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	t, err := time.Parse("2006-01-02T15:04Z07:00", s)
	if err != nil {
		t, err = time.Parse(time.RFC3339, s)
	}
	if err != nil {
		return time.Time{}, invalid("time %q", s)
	}

	return t.UTC(), nil
}

// parseResolution reads PT<n>M and PT<n>H, up to a day: ENTSO-E's P1D, P7D, P1M and P1Y are calendar spans, which
// no series read here has.
func parseResolution(s string) (time.Duration, error) {
	v, ok := strings.CutPrefix(strings.TrimSpace(s), "PT")
	if !ok || v == "" {
		return 0, invalid("resolution %q", s)
	}

	unit := time.Minute
	switch v[len(v)-1] {
	case 'M':
	case 'H':
		unit = time.Hour
	default:
		return 0, invalid("resolution %q", s)
	}
	// n is checked before the product, which could overflow.
	n, err := strconv.Atoi(v[:len(v)-1])
	if err != nil || n <= 0 || time.Duration(n) > maxResolution/unit {
		return 0, invalid("resolution %q", s)
	}

	return time.Duration(n) * unit, nil
}

// invalid is a *DocumentError, its reason formatted.
func invalid(format string, a ...any) error {
	return &DocumentError{Reason: fmt.Sprintf(format, a...)}
}

// merge sorts points by start, and drops each overlapping one kept before it: the earliest start wins, the finer
// resolution on a tie.
func merge(points []scraper.Point) []scraper.Point {
	slices.SortStableFunc(points, func(a, b scraper.Point) int {
		if c := a.Start.Compare(b.Start); c != 0 {
			return c
		}

		return cmp.Compare(a.Duration, b.Duration)
	})

	kept := points[:0]
	var end time.Time
	for _, p := range points {
		if len(kept) > 0 && p.Start.Before(end) {
			continue
		}
		kept = append(kept, p)
		end = p.End()
	}

	return kept
}
