// Package scraper holds what its sources share: a Series, values over time, such as a day's prices or a forecast.
//
//	c := entsoe.New(os.Getenv("ENTSOE_TOKEN"))
//	prices, err := c.DayAheadPrices(ctx, entsoe.FR, from, to)
//	...
//	p, ok := prices.At(time.Now()) // the price of the quarter-hour under way
//
// Each Point carries its own Duration, so that a series may mix resolutions, such as hours before October 2025 and
// quarter-hours after, and hold gaps, such as a day not published yet. Times are instants: a local day, such as one
// in Europe/Paris, is the caller's.
//
// Package entsoe reads ENTSO-E's Transparency Platform: day-ahead prices, load and wind and solar forecasts.
package scraper
