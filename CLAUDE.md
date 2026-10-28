# CLAUDE.md

`github.com/elojah/scraper`, the near future of a grid read from where it is published, for `../scheduler` to decide
what devices do: `scraper.Series` (values over time), and `entsoe`, a client of ENTSO-E's Transparency Platform for
France's day-ahead prices, load forecast, and wind and solar forecast. Go 1.26.1, no dependency. A library: storing
and deciding are the scheduler's. Behaviour is in godoc and the README, plans in TODO.md.

## Layout

- `doc.go`, `series.go`: package `scraper`, `Point` and `Series` (`At`, `Between`, `End`), which every source returns.
- `entsoe/`: `client.go` (`Client`, `New`, `get`, which asks and decodes, `redact`), `option.go` (`WithBaseURL`,
  `WithHTTPClient`), `query.go` (`Area`, `FR`, `PSRType`, the three queries and `between`, which trims to
  `[from, to)`), `document.go` (`Decode`, the XML structs, period expansion, `merge`), `errors.go`.
- `testdata/`: ENTSO-E's answers, recorded by `make testdata` (never edit them), and `a44_overlap.xml`, by hand.
  `examples/01-day-ahead/testdata/` holds a copy of the May day, for its `go:embed`.
- `examples/01-day-ahead`: a program, run against its embedded answer, or live with `-live`.

## Commands

```sh
go test -race ./...             # always -race
make check                      # golangci-lint + tests (-shuffle=on -cpu 1,4)
make fuzz                       # FuzzDecode for FUZZ_TIME (30s), minimizing 5s at most: the seeds are up to 60 KB
ENTSOE_TOKEN=… make testdata    # re-record testdata/, then check the tests' expected values
```

Lint (`.golangci.yml`, alarm's) is `default: all` minus a disable list, tests included. Mind `nlreturn`,
`paralleltest` (`t.Parallel()` first), `err113`, `godot`, `funcorder`, `embeddedstructfieldcheck` (embedded fields
first) and gosec's G304: tests read `testdata/` through `fs.ReadFile` on an `os.DirFS`. CI runs `make check` with a
pinned golangci-lint in `./bin`.

## Conventions

- Comments are short: invariants, non-obvious whys, the public contract.
- Options have no prefix: `WithBaseURL`, `WithHTTPClient`.
- Errors are typed structs with an `Is` method, as sunspec's: `*AcknowledgementError`, `*StatusError`,
  `*DocumentError`, matching the `Err` sentinels. `invalid` builds a `*DocumentError`; `series` prefixes its reason with
  the TimeSeries and Period. Messages start with `entsoe: `.
- XML tags name no namespace, so that any version of a document matches.

## Invariants

- **The token never leaves the request**: `redact` replaces the URL of a `*url.Error`, from `NewRequestWithContext` or
  `Do`, by the base URL. No other error holds a URL. `make testdata` passes it as `$$ENTSOE_TOKEN` under `$Q`, and the
  answers do not hold it. Tests use a fake one.
- **Points are UTC instants**, each a position of its period at its resolution: an A03 position left out repeats the
  previous one up to the next, the last up to the period's end; an A01 one is a gap. Local days are the caller's.
- **`Decode` expands at most `maxPoints` (2^18)**, checked before allocating a period, so that a document of a period
  of years at PT1M costs nothing: ENTSO-E answers a year at most, 105 120 quarter-hours for A69's three series.
  Resolutions are `PT<n>M` and `PT<n>H` up to a day; calendar ones (P1D, P1M) are refused.
- **`merge` sorts stably by start, then duration**, and drops a point starting before the last kept one ends: the
  earliest start wins, the finer resolution on a tie, the document's order after.
- A query returns `ErrNoData` once trimmed to nothing: ENTSO-E answers whole days, and an acknowledgement only when it
  has none at all. `WindSolarForecast` drops the kinds trimmed to nothing.
- `get` reads at most `maxBody` (256 MB). A non-200 answer is an `*AcknowledgementError` if its body is one, with the
  status set, else a `*StatusError`.

## Tests

- External test packages only; `examples/01-day-ahead` is `package main`. Name a test after its method, with a suffix
  for a case: `TestDayAheadPrices_DSTEnd`.
- `serve` runs an `httptest.Server` answering a status and a `testdata/` file, or a literal body, and keeps the last
  query, which `checkQuery` compares whole. `checkSeries` checks points back to back from a start.
- Fixtures cover the local days of 27 and 28 October 2026, 25 October (25 h, 77 of 100 positions), 29 March (23 h),
  10 May (negative prices), ENTSO-E's acknowledgements of no data and of a bad token. Expected values come from the
  XML: re-recording may change them.
- `FuzzDecode` seeds every fixture and checks that a refusal is typed and that series are sorted, finite and do not
  overlap.
- The example has an `Example()` with `// Output:`, from its embedded answer.
