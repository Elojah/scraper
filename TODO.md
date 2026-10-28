# TODO

## scheduler

- [ ] An operation fetching tomorrow on a rule, such as every 20 minutes from 13:00 to 18:00 Europe/Paris, until
  `Series.End()` reaches tomorrow's end, keeping the series in its SQLite: prices, load, wind and solar.
- [ ] A planning step turning a day's series into device schedules, such as `WMaxLimPct` at 0 over the spans below
  zero, through sunspec's model 123.

## Sources

- [ ] ENTSO-E's intraday auctions (IDA1-3): A44 with `contract_MarketAgreement.type=A07`, and a sequence to pick.
- [ ] RTE's Tempo days and EcoWatt signals, behind a `scraper.Series` each.
- [ ] Irradiance forecast, such as Open-Meteo's, for a site's own production.
- [ ] `Client` asking a range over a year in chunks: ENTSO-E refuses more than a year a request.
