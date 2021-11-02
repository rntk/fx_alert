package patterns

import (
	"math"

	"fx_alert/pkg/quoter"
)

const StarBarPercent = 33.0

func starBar(q quoter.Quote) *Pattern {
	h := q.High - q.Low

	t := q.High - math.Max(q.Open, q.Close)
	topP := (t * 100) / h

	t = math.Min(q.Open, q.Close) - q.Low
	bottomP := (t * 100) / h
	if (topP >= StarBarPercent) && (bottomP >= StarBarPercent) {
		return &Pattern{Name: StarBar, Sentiment: Neutral}
	}

	return nil
}
