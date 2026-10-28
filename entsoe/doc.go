// Package entsoe reads ENTSO-E's Transparency Platform: day-ahead prices, load forecast, and wind and solar
// generation forecast, of a bidding zone such as FR.
//
//	c := entsoe.New(os.Getenv("ENTSOE_TOKEN"))
//	prices, err := c.DayAheadPrices(ctx, entsoe.FR, from, to) // EUR/MWh
//	load, err := c.LoadForecast(ctx, entsoe.FR, from, to)     // MW
//	gen, err := c.WindSolarForecast(ctx, entsoe.FR, from, to) // MW, by entsoe.Solar, WindOffshore, WindOnshore
//
// A security token comes with an account on transparency.entsoe.eu, once transparency@entsoe.eu, asked by mail with
// "Restful API access" as subject, granted it. ENTSO-E takes 400 requests a minute per token.
//
// Each query returns the points starting in [from, to), each a position of its period at its resolution, such as a
// quarter-hour: ENTSO-E answers whole days. A range only partly published returns what is, which Series.End tells,
// and one with nothing ErrNoData. A curve of type A03 leaves out the positions repeating the previous value: they are
// filled back in.
//
// A refusal ENTSO-E explains is an *AcknowledgementError, another HTTP status a *StatusError, and an answer that is
// no document Decode reads a *DocumentError. They match ErrNoData, ErrUnauthorized, ErrRateLimited and
// ErrInvalidDocument with errors.Is. No error holds the token.
package entsoe
