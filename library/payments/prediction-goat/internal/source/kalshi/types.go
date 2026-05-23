package kalshi

import "encoding/json"

type Market struct {
	Ticker         string  `json:"ticker"`
	EventTicker    string  `json:"event_ticker"`
	SeriesTicker   string  `json:"series_ticker"`
	Title          string  `json:"title"`
	Status         string  `json:"status"`
	YesAsk         float64 `json:"yes_ask_dollars"`
	NoAsk          float64 `json:"no_ask_dollars"`
	YesBid         float64 `json:"yes_bid_dollars"`
	NoBid          float64 `json:"no_bid_dollars"`
	LastPrice      float64 `json:"last_price_dollars"`
	Volume24h      float64 `json:"volume_24h_fp"`
	Liquidity      float64 `json:"liquidity_dollars"`
	OpenTime       string  `json:"open_time"`
	CloseTime      string  `json:"close_time"`
	ExpirationTime string  `json:"expiration_time"`
}

type Event struct {
	EventTicker       string `json:"event_ticker"`
	SeriesTicker      string `json:"series_ticker"`
	Title             string `json:"title"`
	SubTitle          string `json:"sub_title"`
	Category          string `json:"category"`
	StrikePeriod      string `json:"strike_period"`
	MutuallyExclusive bool   `json:"mutually_exclusive"`
}

type Series struct {
	Ticker    string   `json:"ticker"`
	Title     string   `json:"title"`
	Category  string   `json:"category"`
	Frequency string   `json:"frequency"`
	Tags      []string `json:"tags"`
}

type MarketsResponse struct {
	Markets []json.RawMessage `json:"markets"`
	Cursor  string            `json:"cursor"`
}

type EventsResponse struct {
	Events []json.RawMessage `json:"events"`
	Cursor string            `json:"cursor"`
}

type SeriesResponse struct {
	Series []json.RawMessage `json:"series"`
	Cursor string            `json:"cursor"`
}
