package entsoe

import (
	"context"
	"net/url"
	"time"

	"github.com/elojah/scraper"
)

// Area is a bidding zone's EIC code.
type Area string

// FR is France's bidding zone.
const FR Area = "10YFR-RTE------C"

// PSRType is a kind of production, as ENTSO-E codes it.
type PSRType string

// The kinds of production WindSolarForecast returns.
const (
	Solar        PSRType = "B16"
	WindOffshore PSRType = "B18"
	WindOnshore  PSRType = "B19"
)

// DayAheadPrices returns area's day-ahead prices (12.1.D) starting in [from, to), in EUR/MWh: the results of SDAC's
// auction, out at about 12:45 CET for the next day, quarter-hourly since 1 October 2025.
func (c *Client) DayAheadPrices(ctx context.Context, area Area, from, to time.Time) (scraper.Series, error) {
	series, err := c.get(ctx, url.Values{
		"documentType":                  {"A44"},
		"in_Domain":                     {string(area)},
		"out_Domain":                    {string(area)},
		"contract_MarketAgreement.type": {"A01"},
	}, from, to)
	if err != nil {
		return scraper.Series{}, err
	}

	return between(series[""], from, to)
}

// LoadForecast returns area's day-ahead total load forecast (6.1.B) starting in [from, to), in MW.
func (c *Client) LoadForecast(ctx context.Context, area Area, from, to time.Time) (scraper.Series, error) {
	series, err := c.get(ctx, url.Values{
		"documentType":          {"A65"},
		"processType":           {"A01"},
		"outBiddingZone_Domain": {string(area)},
	}, from, to)
	if err != nil {
		return scraper.Series{}, err
	}

	return between(series[""], from, to)
}

// WindSolarForecast returns area's day-ahead wind and solar generation forecast (14.1.D) starting in [from, to), in
// MW, by kind of production: Solar, WindOffshore and WindOnshore, those area publishes.
func (c *Client) WindSolarForecast(ctx context.Context, area Area, from, to time.Time) (map[PSRType]scraper.Series, error) {
	series, err := c.get(ctx, url.Values{
		"documentType": {"A69"},
		"processType":  {"A01"},
		"in_Domain":    {string(area)},
	}, from, to)
	if err != nil {
		return nil, err
	}

	for k, s := range series {
		s, err := between(s, from, to)
		if err != nil {
			delete(series, k)

			continue
		}
		series[k] = s
	}
	if len(series) == 0 {
		return nil, ErrNoData
	}

	return series, nil
}

// between returns the points of s starting in [from, to), ErrNoData if none: ENTSO-E answers whole days.
func between(s scraper.Series, from, to time.Time) (scraper.Series, error) {
	s = s.Between(from, to)
	if len(s.Points) == 0 {
		return scraper.Series{}, ErrNoData
	}

	return s, nil
}
