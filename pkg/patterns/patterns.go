package patterns

import "fx_alert/pkg/quoter"

type Name string

const (
	PinBar  Name = "pinbar"
	StarBar Name = "starbar"
)

type Sentiment string

const (
	Bull    Sentiment = "bull"
	Bear    Sentiment = "bear"
	Neutral Sentiment = "neutral"
)

type Pattern struct {
	Name      Name
	Sentiment Sentiment
}

func FindPattern(q quoter.Quote) *Pattern {
	if p := pinBar(q); p != nil {
		return p
	}
	if p := starBar(q); p != nil {
		return p
	}

	return nil
}
