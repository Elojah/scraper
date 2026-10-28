package entsoe_test

import (
	"bytes"
	"errors"
	"io/fs"
	"math"
	"testing"

	"github.com/elojah/scraper/entsoe"
)

// FuzzDecode checks that Decode refuses with an *AcknowledgementError or a *DocumentError, and that what it reads is
// series of points sorted, none overlapping, of finite values.
func FuzzDecode(f *testing.F) {
	files, err := fs.Glob(testdata, "*.xml")
	if err != nil {
		f.Fatal(err)
	}
	for _, file := range files {
		b, err := fs.ReadFile(testdata, file)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(b)
	}
	f.Add([]byte(prices("A03", "2026-10-26T23:00Z", "2026-10-27T00:00Z", "PT15M", pt(1, "1"), pt(3, "3"))))

	f.Fuzz(func(t *testing.T, b []byte) {
		series, err := entsoe.Decode(bytes.NewReader(b))
		if err != nil {
			var ack *entsoe.AcknowledgementError
			if !errors.As(err, &ack) && !errors.Is(err, entsoe.ErrInvalidDocument) {
				t.Fatalf("%T, want an *AcknowledgementError or a *DocumentError", err)
			}

			return
		}

		for k, s := range series {
			for i, p := range s.Points {
				if p.Duration <= 0 || math.IsNaN(p.Value) || math.IsInf(p.Value, 0) {
					t.Fatalf("%s point %d: %+v", k, i, p)
				}
				if i > 0 && p.Start.Before(s.Points[i-1].End()) {
					t.Fatalf("%s point %d at %s, before %s", k, i, p.Start, s.Points[i-1].End())
				}
			}
		}
	})
}
