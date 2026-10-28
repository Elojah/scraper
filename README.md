<p align="center">
  <img src="assets/logo.svg" alt="scraper" width="180">
</p>

# scraper

[![CI](https://github.com/Elojah/scraper/actions/workflows/ci.yml/badge.svg)](https://github.com/Elojah/scraper/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/elojah/scraper.svg)](https://pkg.go.dev/github.com/elojah/scraper)

The near future of a grid, read from where it is published, for a scheduler to decide what devices do: France's
day-ahead prices, load forecast, and wind and solar forecast, from ENTSO-E's Transparency Platform, as series of
values over time. No dependencies.

```sh
go get github.com/elojah/scraper
```

## Quick start

```go
paris, _ := time.LoadLocation("Europe/Paris")
day := time.Date(2026, 10, 29, 0, 0, 0, 0, paris)

c := entsoe.New(os.Getenv("ENTSOE_TOKEN"))
prices, err := c.DayAheadPrices(ctx, entsoe.FR, day, day.AddDate(0, 0, 1))
if errors.Is(err, entsoe.ErrNoData) {
	// not published yet: about 12:45 CET the day before
}
...
p, ok := prices.At(time.Date(2026, 10, 29, 19, 0, 0, 0, paris)) // 391.83 EUR/MWh, over 19:00-19:15
```

## Series

A `scraper.Series` is values in a `Unit` over `Points`, sorted, none overlapping another, each over
`[Start, Start+Duration)`:

| Method | Does |
| --- | --- |
| `s.At(t)` | The point whose span holds `t`, false in a gap. |
| `s.Between(from, to)` | The points starting in `[from, to)`, sharing `s`'s array. |
| `s.End()` | When the last point stops: tomorrow's prices are in once it reaches tomorrow's end. |

Each point carries its own duration, so a series mixes resolutions, hours before October 2025 and quarter-hours
after, and holds gaps. Times are instants: a local day is the caller's, 23 or 25 hours when clocks change.

## ENTSO-E

| Method | Data item | Unit | Request |
| --- | --- | --- | --- |
| `DayAheadPrices` | 12.1.D, SDAC's auction, out at about 12:45 CET for the next day | EUR/MWh | `documentType=A44`, `in_Domain` and `out_Domain`, `contract_MarketAgreement.type=A01` |
| `LoadForecast` | 6.1.B, day-ahead total load forecast | MW | `documentType=A65`, `processType=A01`, `outBiddingZone_Domain` |
| `WindSolarForecast` | 14.1.D, day-ahead, by `entsoe.Solar`, `WindOffshore` and `WindOnshore` (B16, B18, B19) | MW | `documentType=A69`, `processType=A01`, `in_Domain` |

`entsoe.FR` is France's bidding zone, `10YFR-RTE------C`; any other EIC code is an `entsoe.Area`. Each method returns
the points starting in `[from, to)`: ENTSO-E answers whole days. A range only partly published returns what is, and
one with nothing `ErrNoData`. `entsoe.Decode` reads a document saved from the platform the same way.

A security token comes with an account on [transparency.entsoe.eu](https://transparency.entsoe.eu/), once
transparency@entsoe.eu, asked by mail with "Restful API access" as subject, granted it. ENTSO-E takes 400 requests a
minute per token. The platform is migrating to a new one: `entsoe.WithBaseURL` points a Client elsewhere than
`entsoe.BaseURL`.

What ENTSO-E's documents hold, as seen for France in October 2026:

- Every series is quarter-hourly, with curve type A03: a position repeating the previous value is left out, 23 of
  the 100 of 25 October, when clocks went back. They are filled back in, each position a point.
- Several TimeSeries over the same span would overlap: the earliest start wins, the finer resolution on a tie.
- The solar forecast of 27 and 28 October is a single point at 0 over both days, July's peaks at 16 GW: what
  ENTSO-E publishes is returned as is.

## Errors

| Error | When |
| --- | --- |
| `*entsoe.AcknowledgementError` | ENTSO-E refused, explaining why: its `Code` is 999 whatever the reason, its `Text` the reason. |
| `*entsoe.StatusError` | An HTTP status other than 200, without an explanation. |
| `*entsoe.DocumentError` | An answer that is no document `Decode` reads. Matches `entsoe.ErrInvalidDocument`. |
| `entsoe.ErrNoData` | Nothing of the range is published. Matched by an acknowledgement saying so, or returned. |
| `entsoe.ErrUnauthorized`, `entsoe.ErrRateLimited` | Matched by an acknowledgement or a status, 401 or 429. |

No error holds the token: a request's URL is replaced by the base URL in a `*url.Error`.

## Examples

| Example | Shows |
| --- | --- |
| [`01-day-ahead`](examples/01-day-ahead/main.go) | France's prices of a Sunday, served in process from ENTSO-E's recorded answer: the price at 19:00, the cheapest two hours, the spans below zero. `-live` asks for tomorrow's, with `ENTSOE_TOKEN`. |

## Development

```sh
make check                      # lint, and tests with -race
make fuzz                       # FuzzDecode for FUZZ_TIME (30s)
ENTSOE_TOKEN=… make testdata    # record ENTSO-E's answers into testdata/
```

## License

MIT.
