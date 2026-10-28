package entsoe_test

import (
	"bytes"
	"errors"
	"io/fs"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/elojah/scraper/entsoe"
)

// prices is a Publication_MarketDocument of a TimeSeries of one Period, of points made by pt.
func prices(curve, start, end, resolution string, points ...string) string {
	return `<?xml version="1.0" encoding="utf-8"?>
<Publication_MarketDocument xmlns="urn:iec62325.351:tc57wg16:451-3:publicationdocument:7:3"><TimeSeries>` +
		`<currency_Unit.name>EUR</currency_Unit.name><price_Measure_Unit.name>MWH</price_Measure_Unit.name>` +
		`<curveType>` + curve + `</curveType><Period><timeInterval><start>` + start + `</start><end>` + end +
		`</end></timeInterval><resolution>` + resolution + `</resolution>` + strings.Join(points, "") +
		`</Period></TimeSeries></Publication_MarketDocument>`
}

func pt(position int, value string) string {
	return "<Point><position>" + strconv.Itoa(position) + "</position><price.amount>" + value + "</price.amount></Point>"
}

func decode(t *testing.T, file string) map[entsoe.PSRType][]float64 {
	t.Helper()

	b, err := fs.ReadFile(testdata, file)
	if err != nil {
		t.Fatal(err)
	}

	series, err := entsoe.Decode(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	values := make(map[entsoe.PSRType][]float64, len(series))
	for k, s := range series {
		for _, p := range s.Points {
			values[k] = append(values[k], p.Value)
		}
	}

	return values
}

// TestDecode_Overlap checks that of points overlapping, the earliest start wins, the finer resolution on a tie.
func TestDecode_Overlap(t *testing.T) {
	t.Parallel()

	got := decode(t, "a44_overlap.xml")[""]
	want := []float64{10, 20, 20, 40, 200}
	if len(got) != len(want) {
		t.Fatalf("%v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%v, want %v", got, want)
		}
	}
}

// TestDecode_Fixed checks that an A01 curve leaves a position left out a gap.
func TestDecode_Fixed(t *testing.T) {
	t.Parallel()

	series, err := entsoe.Decode(strings.NewReader(prices("A01", "2026-10-26T23:00Z", "2026-10-27T00:00Z", "PT15M",
		pt(1, "1.5"), pt(3, "-3"), pt(4, "4"))))
	if err != nil {
		t.Fatal(err)
	}

	s := series[""]
	if len(s.Points) != 3 {
		t.Fatalf("%d points, want 3: %v", len(s.Points), s.Points)
	}
	if _, ok := s.At(oct27.Add(20 * time.Minute)); ok {
		t.Fatal("position 2 has a point")
	}
	if p, ok := s.At(oct27.Add(30 * time.Minute)); !ok || p.Value != -3 {
		t.Fatalf("position 3 is %v, %t, want -3", p.Value, ok)
	}
}

// TestDecode_Hourly checks PT60M and PT1H, and a time with seconds.
func TestDecode_Hourly(t *testing.T) {
	t.Parallel()

	for _, resolution := range []string{"PT60M", "PT1H"} {
		series, err := entsoe.Decode(strings.NewReader(prices("A03", "2026-10-26T23:00:00Z", "2026-10-27T02:00Z", resolution,
			pt(1, "1"))))
		if err != nil {
			t.Fatal(err)
		}
		checkSeries(t, series[""], "EUR/MWh", oct27, 3, time.Hour)
	}
}

func TestDecode_Acknowledgement(t *testing.T) {
	t.Parallel()

	_, err := entsoe.Decode(strings.NewReader(`<Acknowledgement_MarketDocument><mRID>1</mRID></Acknowledgement_MarketDocument>`))
	var ack *entsoe.AcknowledgementError
	if !errors.As(err, &ack) || ack.StatusCode != 0 || errors.Is(err, entsoe.ErrNoData) {
		t.Fatalf("%#v, want an *AcknowledgementError without reason", err)
	}
}

func TestDecode_Invalid(t *testing.T) {
	t.Parallel()

	const start, end = "2026-10-26T23:00Z", "2026-10-27T00:00Z"
	for _, c := range []struct {
		name string
		doc  string
	}{
		{"empty", ""},
		{"syntax", "<Publication_MarketDocument><TimeSeries>"},
		{"root", "<html><body>Hello</body></html>"},
		{"curve type", prices("A05", start, end, "PT15M", pt(1, "1"))},
		{"calendar resolution", prices("A03", start, end, "P1D", pt(1, "1"))},
		{"zero resolution", prices("A03", start, end, "PT0M", pt(1, "1"))},
		{"resolution past a day", prices("A03", start, end, "PT25H", pt(1, "1"))},
		{"resolution unit", prices("A03", start, end, "PT15S", pt(1, "1"))},
		{"resolution number", prices("A03", start, end, "PTxM", pt(1, "1"))},
		{"start", prices("A03", "yesterday", end, "PT15M", pt(1, "1"))},
		{"end", prices("A03", start, "tomorrow", "PT15M", pt(1, "1"))},
		{"end before start", prices("A03", end, start, "PT15M", pt(1, "1"))},
		{"not whole", prices("A03", start, "2026-10-26T23:20Z", "PT15M", pt(1, "1"))},
		{"no point", prices("A03", start, end, "PT15M")},
		{"position 0", prices("A01", start, end, "PT15M", pt(0, "1"))},
		{"position past the end", prices("A01", start, end, "PT15M", pt(1, "1"), pt(5, "5"))},
		{"positions back", prices("A03", start, end, "PT15M", pt(1, "1"), pt(3, "3"), pt(2, "2"))},
		{"positions twice", prices("A01", start, end, "PT15M", pt(1, "1"), pt(1, "1"))},
		{"first position", prices("A03", start, end, "PT15M", pt(2, "1"))},
		{"value", prices("A03", start, end, "PT15M", pt(1, "abc"))},
		{"no value", prices("A03", start, end, "PT15M", pt(1, ""))},
		{"NaN", prices("A03", start, end, "PT15M", pt(1, "NaN"))},
		{"Inf", prices("A03", start, end, "PT15M", pt(1, "+Inf"))},
		{"too many points", prices("A03", "2026-01-01T00:00Z", "2027-01-01T00:00Z", "PT1M", pt(1, "1"))},
		{"units", strings.Replace(prices("A03", start, end, "PT15M", pt(1, "1")), "</TimeSeries>",
			"</TimeSeries><TimeSeries><quantity_Measure_Unit.name>MAW</quantity_Measure_Unit.name></TimeSeries>", 1)},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			_, err := entsoe.Decode(strings.NewReader(c.doc))
			var de *entsoe.DocumentError
			if !errors.As(err, &de) || !errors.Is(err, entsoe.ErrInvalidDocument) {
				t.Fatalf("%v, want a *DocumentError", err)
			}
		})
	}
}
