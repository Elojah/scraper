// France's day-ahead prices of Sunday 10 May 2026, served in process from ENTSO-E's recorded answer: the price at
// 19:00, the cheapest two hours to run a load in, and the spans paid to consume. With -live, tomorrow's, from ENTSO-E
// with ENTSOE_TOKEN.
//
// `go run ./examples/01-day-ahead`, `ENTSOE_TOKEN=… go run ./examples/01-day-ahead -live`.
package main

import (
	"context"
	_ "embed"
	"errors"
	"flag"
	"fmt"
	"log"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"time"
	_ "time/tzdata" // Europe/Paris on a host without a zoneinfo database

	"github.com/elojah/scraper"
	"github.com/elojah/scraper/entsoe"
)

//go:embed testdata/a44_fr_20260510.xml
var recorded []byte

var live = flag.Bool("live", false, "ask ENTSO-E for tomorrow's prices, with ENTSOE_TOKEN")

func main() {
	flag.Parse()

	paris, err := time.LoadLocation("Europe/Paris")
	if err != nil {
		log.Fatal(err)
	}

	day := time.Date(2026, 5, 10, 0, 0, 0, 0, paris)
	var client *entsoe.Client
	if *live {
		token := os.Getenv("ENTSOE_TOKEN")
		if token == "" {
			log.Fatal("ENTSOE_TOKEN is not set")
		}
		now := time.Now().In(paris)
		day = time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, paris)
		client = entsoe.New(token)
	} else {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write(recorded)
		}))
		defer server.Close()
		client = entsoe.New("recorded", entsoe.WithBaseURL(server.URL))
	}

	// A local day, 23 or 25 hours when clocks change.
	prices, err := client.DayAheadPrices(context.Background(), entsoe.FR, day, day.AddDate(0, 0, 1))
	if errors.Is(err, entsoe.ErrNoData) {
		fmt.Println(day.Format("Monday 2 January 2006"), "is not published yet: about 12:45 CET the day before")

		return
	}
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("%s, France: %d prices of %s, in %s\n",
		day.Format("Monday 2 January 2006"), len(prices.Points), prices.Points[0].Duration, prices.Unit)
	if p, ok := prices.At(time.Date(day.Year(), day.Month(), day.Day(), 19, 0, 0, 0, paris)); ok {
		fmt.Printf("at 19:00: %.2f\n", p.Value)
	}
	if w, ok := cheapest(prices, 2*time.Hour); ok {
		fmt.Printf("cheapest 2h: %s to %s, %.2f on average\n",
			w.Start.In(paris).Format("15:04"), w.End().In(paris).Format("15:04"), w.Value)
	}
	for _, span := range negative(prices) {
		fmt.Printf("below zero: %s to %s, down to %.2f\n",
			span.Start.In(paris).Format("15:04"), span.End().In(paris).Format("15:04"), span.Value)
	}
}

// cheapest returns the span of d, over back-to-back points, of the lowest mean price.
func cheapest(s scraper.Series, d time.Duration) (scraper.Point, bool) {
	best := scraper.Point{Value: math.Inf(1)}
	for i, first := range s.Points {
		var sum float64
		end := first.Start
		for _, p := range s.Points[i:] {
			if !p.Start.Equal(end) || !end.Before(first.Start.Add(d)) {
				break
			}
			sum += p.Value * p.Duration.Hours()
			end = p.End()
		}
		if mean := sum / d.Hours(); end.Sub(first.Start) == d && mean < best.Value {
			best = scraper.Point{Start: first.Start, Duration: d, Value: mean}
		}
	}

	return best, !math.IsInf(best.Value, 1)
}

// negative returns the spans of back-to-back points priced below zero, each with its lowest price.
func negative(s scraper.Series) []scraper.Point {
	var spans []scraper.Point
	for _, p := range s.Points {
		if p.Value >= 0 {
			continue
		}
		if n := len(spans); n > 0 && spans[n-1].End().Equal(p.Start) {
			spans[n-1].Duration += p.Duration
			spans[n-1].Value = min(spans[n-1].Value, p.Value)

			continue
		}
		spans = append(spans, p)
	}

	return spans
}
