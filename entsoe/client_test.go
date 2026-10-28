package entsoe_test

import (
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/elojah/scraper"
	"github.com/elojah/scraper/entsoe"
)

const token = "f00dcafe-0000-4000-8000-000000000000"

// Local days in Europe/Paris, as UTC: 27 Oct 2026, the 25 h day clocks went back, and the 23 h one they went forward.
var (
	oct27 = time.Date(2026, 10, 26, 23, 0, 0, 0, time.UTC)
	oct29 = time.Date(2026, 10, 28, 23, 0, 0, 0, time.UTC)
	oct25 = time.Date(2026, 10, 24, 22, 0, 0, 0, time.UTC)
	oct26 = time.Date(2026, 10, 25, 23, 0, 0, 0, time.UTC)
	mar29 = time.Date(2026, 3, 28, 23, 0, 0, 0, time.UTC)
	mar30 = time.Date(2026, 3, 29, 22, 0, 0, 0, time.UTC)
)

// testdata is the repository's testdata/.
var testdata = os.DirFS(filepath.Join("..", "testdata"))

// server answers each request with a status and a body, and keeps the last query it got.
type server struct {
	mu    sync.Mutex
	query url.Values
}

func (s *server) last() url.Values {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.query
}

// serve returns a Client of a server answering status and testdata/file, or file itself if it is no file name.
func serve(t *testing.T, status int, file string) (*entsoe.Client, *server) {
	t.Helper()

	body := []byte(file)
	if strings.HasSuffix(file, ".xml") {
		var err error
		if body, err = fs.ReadFile(testdata, file); err != nil {
			t.Fatal(err)
		}
	}

	s := &server{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.query = r.URL.Query()
		s.mu.Unlock()
		w.Header().Set("Content-Type", "text/xml")
		w.WriteHeader(status)
		_, _ = w.Write(body)
	}))
	t.Cleanup(ts.Close)

	return entsoe.New(token, entsoe.WithBaseURL(ts.URL), entsoe.WithHTTPClient(ts.Client())), s
}

// checkQuery checks that q holds the token, the range and want.
func checkQuery(t *testing.T, q url.Values, from, to string, want map[string]string) {
	t.Helper()

	want["securityToken"] = token
	want["periodStart"] = from
	want["periodEnd"] = to
	for k, v := range want {
		if got := q.Get(k); got != v {
			t.Errorf("%s=%q, want %q", k, got, v)
		}
	}
	if len(q) != len(want) {
		t.Errorf("query %v, want only %v", q, want)
	}
}

// checkSeries checks that s is n points of d in unit, back to back from from.
func checkSeries(t *testing.T, s scraper.Series, unit string, from time.Time, n int, d time.Duration) {
	t.Helper()

	if s.Unit != unit {
		t.Errorf("Unit %q, want %q", s.Unit, unit)
	}
	if len(s.Points) != n {
		t.Fatalf("%d points, want %d", len(s.Points), n)
	}
	at := from
	for i, p := range s.Points {
		if !p.Start.Equal(at) || p.Duration != d {
			t.Fatalf("point %d at %s for %s, want at %s for %s", i, p.Start, p.Duration, at, d)
		}
		at = p.End()
	}
}

func TestDayAheadPrices(t *testing.T) {
	t.Parallel()

	c, s := serve(t, http.StatusOK, "a44_fr_20261027.xml")
	got, err := c.DayAheadPrices(t.Context(), entsoe.FR, oct27, oct29)
	if err != nil {
		t.Fatal(err)
	}

	checkQuery(t, s.last(), "202610262300", "202610282300", map[string]string{
		"documentType":                  "A44",
		"in_Domain":                     string(entsoe.FR),
		"out_Domain":                    string(entsoe.FR),
		"contract_MarketAgreement.type": "A01",
	})
	checkSeries(t, got, "EUR/MWh", oct27, 192, 15*time.Minute)
	if got.Points[0].Value != 135.33 || got.Points[96].Value != 229.25 || got.Points[191].Value != 224.88 {
		t.Fatalf("points 0, 96 and 191 are %v, %v and %v, want 135.33, 229.25 and 224.88",
			got.Points[0].Value, got.Points[96].Value, got.Points[191].Value)
	}
}

func TestDayAheadPrices_Between(t *testing.T) {
	t.Parallel()

	c, _ := serve(t, http.StatusOK, "a44_fr_20261027.xml")
	from := oct27.Add(12 * time.Hour)
	got, err := c.DayAheadPrices(t.Context(), entsoe.FR, from, from.Add(2*time.Hour))
	if err != nil {
		t.Fatal(err)
	}

	checkSeries(t, got, "EUR/MWh", from, 8, 15*time.Minute)
}

// TestDayAheadPrices_DSTEnd checks the 25 h day, whose positions left out repeat the previous one.
func TestDayAheadPrices_DSTEnd(t *testing.T) {
	t.Parallel()

	c, _ := serve(t, http.StatusOK, "a44_fr_20261025.xml")
	got, err := c.DayAheadPrices(t.Context(), entsoe.FR, oct25, oct26)
	if err != nil {
		t.Fatal(err)
	}

	checkSeries(t, got, "EUR/MWh", oct25, 100, 15*time.Minute)
	// Positions 12, 15 and 16 are left out.
	if p := got.Points; p[11].Value != p[10].Value || p[14].Value != p[13].Value || p[15].Value != p[13].Value {
		t.Fatalf("positions 11 to 16 are %v, want 12 as 11, 15 and 16 as 14", p[10:16])
	}
	if got.Points[0].Value != 24.77 || got.Points[99].Value != 27.19 {
		t.Fatalf("positions 1 and 100 are %v and %v, want 24.77 and 27.19", got.Points[0].Value, got.Points[99].Value)
	}
}

// TestDayAheadPrices_DSTStart checks the 23 h day.
func TestDayAheadPrices_DSTStart(t *testing.T) {
	t.Parallel()

	c, _ := serve(t, http.StatusOK, "a44_fr_20260329.xml")
	got, err := c.DayAheadPrices(t.Context(), entsoe.FR, mar29, mar30)
	if err != nil {
		t.Fatal(err)
	}

	checkSeries(t, got, "EUR/MWh", mar29, 92, 15*time.Minute)
}

// TestDayAheadPrices_OutOfRange checks a document of days other than those asked.
func TestDayAheadPrices_OutOfRange(t *testing.T) {
	t.Parallel()

	c, _ := serve(t, http.StatusOK, "a44_fr_20261027.xml")
	_, err := c.DayAheadPrices(t.Context(), entsoe.FR, oct29, oct29.Add(24*time.Hour))
	if !errors.Is(err, entsoe.ErrNoData) {
		t.Fatalf("%v, want ErrNoData", err)
	}
}

func TestDayAheadPrices_NoData(t *testing.T) {
	t.Parallel()

	c, _ := serve(t, http.StatusOK, "ack_nodata.xml")
	_, err := c.DayAheadPrices(t.Context(), entsoe.FR, oct27, oct29)
	if !errors.Is(err, entsoe.ErrNoData) || errors.Is(err, entsoe.ErrUnauthorized) {
		t.Fatalf("%v, want ErrNoData only", err)
	}
	var ack *entsoe.AcknowledgementError
	if !errors.As(err, &ack) || ack.StatusCode != http.StatusOK || ack.Code != "999" {
		t.Fatalf("%#v, want an *AcknowledgementError, 200, 999", err)
	}
}

func TestDayAheadPrices_Unauthorized(t *testing.T) {
	t.Parallel()

	c, _ := serve(t, http.StatusUnauthorized, "ack_auth.xml")
	_, err := c.DayAheadPrices(t.Context(), entsoe.FR, oct27, oct29)
	if !errors.Is(err, entsoe.ErrUnauthorized) || errors.Is(err, entsoe.ErrNoData) {
		t.Fatalf("%v, want ErrUnauthorized only", err)
	}
	var ack *entsoe.AcknowledgementError
	if !errors.As(err, &ack) || ack.Text != "Authentication failed." {
		t.Fatalf("%#v, want an *AcknowledgementError, Authentication failed.", err)
	}
}

func TestDayAheadPrices_Status(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		name   string
		status int
		body   string
		want   error
	}{
		{"rate limited", http.StatusTooManyRequests, "Too many requests", entsoe.ErrRateLimited},
		{"unauthorized", http.StatusUnauthorized, "", entsoe.ErrUnauthorized},
		{"unavailable", http.StatusServiceUnavailable, "<html><body>Maintenance</body></html>", nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			client, _ := serve(t, c.status, c.body)
			_, err := client.DayAheadPrices(t.Context(), entsoe.FR, oct27, oct29)
			var se *entsoe.StatusError
			if !errors.As(err, &se) || se.StatusCode != c.status {
				t.Fatalf("%v, want a *StatusError %d", err, c.status)
			}
			for _, target := range []error{entsoe.ErrRateLimited, entsoe.ErrUnauthorized} {
				if want := errors.Is(c.want, target); errors.Is(err, target) != want {
					t.Fatalf("errors.Is(%v, %v) is %t", err, target, !want)
				}
			}
		})
	}
}

func TestDayAheadPrices_Invalid(t *testing.T) {
	t.Parallel()

	c, _ := serve(t, http.StatusOK, "<html><body>Hello</body></html>")
	_, err := c.DayAheadPrices(t.Context(), entsoe.FR, oct27, oct29)
	if !errors.Is(err, entsoe.ErrInvalidDocument) {
		t.Fatalf("%v, want ErrInvalidDocument", err)
	}
}

// TestDayAheadPrices_Redacted checks that a request failing keeps the token out of its error.
func TestDayAheadPrices_Redacted(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.NotFoundHandler())
	ts.Close()

	c := entsoe.New(token, entsoe.WithBaseURL(ts.URL))
	_, err := c.DayAheadPrices(t.Context(), entsoe.FR, oct27, oct29)
	if err == nil {
		t.Fatal("no error from a closed server")
	}
	if strings.Contains(err.Error(), token) {
		t.Fatalf("%v holds the token", err)
	}
	var ue *url.Error
	if !errors.As(err, &ue) || ue.URL != ts.URL {
		t.Fatalf("%#v, want a *url.Error of %s", err, ts.URL)
	}
}

func TestLoadForecast(t *testing.T) {
	t.Parallel()

	c, s := serve(t, http.StatusOK, "a65_fr_20261027.xml")
	got, err := c.LoadForecast(t.Context(), entsoe.FR, oct27, oct29)
	if err != nil {
		t.Fatal(err)
	}

	checkQuery(t, s.last(), "202610262300", "202610282300", map[string]string{
		"documentType":          "A65",
		"processType":           "A01",
		"outBiddingZone_Domain": string(entsoe.FR),
	})
	checkSeries(t, got, "MW", oct27, 192, 15*time.Minute)
	// Positions 18 and 19 are left out, after 17's 40100.
	if p := got.Points; p[0].Value != 49100 || p[16].Value != 40100 || p[17].Value != 40100 || p[18].Value != 40100 {
		t.Fatalf("positions 1, 17, 18 and 19 are %v, %v, %v and %v, want 49100 then 40100",
			p[0].Value, p[16].Value, p[17].Value, p[18].Value)
	}
}

func TestWindSolarForecast(t *testing.T) {
	t.Parallel()

	c, s := serve(t, http.StatusOK, "a69_fr_20261027.xml")
	got, err := c.WindSolarForecast(t.Context(), entsoe.FR, oct27, oct29)
	if err != nil {
		t.Fatal(err)
	}

	checkQuery(t, s.last(), "202610262300", "202610282300", map[string]string{
		"documentType": "A69",
		"processType":  "A01",
		"in_Domain":    string(entsoe.FR),
	})
	if len(got) != 3 {
		t.Fatalf("%d kinds of production, want 3: %v", len(got), got)
	}
	for _, k := range []entsoe.PSRType{entsoe.Solar, entsoe.WindOffshore, entsoe.WindOnshore} {
		checkSeries(t, got[k], "MW", oct27, 192, 15*time.Minute)
	}
	// ENTSO-E's solar forecast of France for these days is one A03 point at 0.
	for i, p := range got[entsoe.Solar].Points {
		if p.Value != 0 {
			t.Fatalf("solar point %d is %v, want 0", i, p.Value)
		}
	}
}

func TestWindSolarForecast_NoData(t *testing.T) {
	t.Parallel()

	c, _ := serve(t, http.StatusOK, "a69_fr_20261027.xml")
	_, err := c.WindSolarForecast(t.Context(), entsoe.FR, oct29, oct29.Add(24*time.Hour))
	if !errors.Is(err, entsoe.ErrNoData) {
		t.Fatalf("%v, want ErrNoData", err)
	}
}

// TestDayAheadPrices_RedactedURL checks that a base URL that does not parse keeps the token out of its error.
func TestDayAheadPrices_RedactedURL(t *testing.T) {
	t.Parallel()

	c := entsoe.New(token, entsoe.WithBaseURL("http://[::1"))
	_, err := c.DayAheadPrices(t.Context(), entsoe.FR, oct27, oct29)
	if err == nil {
		t.Fatal("no error from a base URL that does not parse")
	}
	if strings.Contains(err.Error(), token) {
		t.Fatalf("%v holds the token", err)
	}
}

// TestDayAheadPrices_RateLimited checks an acknowledgement coming with a 429.
func TestDayAheadPrices_RateLimited(t *testing.T) {
	t.Parallel()

	c, _ := serve(t, http.StatusTooManyRequests, "ack_nodata.xml")
	_, err := c.DayAheadPrices(t.Context(), entsoe.FR, oct27, oct29)
	var ack *entsoe.AcknowledgementError
	if !errors.As(err, &ack) || !errors.Is(err, entsoe.ErrRateLimited) || errors.Is(err, entsoe.ErrUnauthorized) {
		t.Fatalf("%v, want an *AcknowledgementError matching ErrRateLimited", err)
	}
}

func TestLoadForecast_NoData(t *testing.T) {
	t.Parallel()

	c, _ := serve(t, http.StatusOK, "ack_nodata.xml")
	_, err := c.LoadForecast(t.Context(), entsoe.FR, oct27, oct29)
	if !errors.Is(err, entsoe.ErrNoData) {
		t.Fatalf("%v, want ErrNoData", err)
	}
}

func TestWindSolarForecast_Unauthorized(t *testing.T) {
	t.Parallel()

	c, _ := serve(t, http.StatusUnauthorized, "ack_auth.xml")
	_, err := c.WindSolarForecast(t.Context(), entsoe.FR, oct27, oct29)
	if !errors.Is(err, entsoe.ErrUnauthorized) {
		t.Fatalf("%v, want ErrUnauthorized", err)
	}
}
