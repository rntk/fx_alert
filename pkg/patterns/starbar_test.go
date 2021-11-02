package patterns

import (
	"reflect"
	"testing"

	"fx_alert/pkg/quoter"
)

func TestStarBar(t *testing.T) {
	type tableData struct {
		q      quoter.Quote
		expect *Pattern
	}

	data := []tableData{
		{
			q:      quoter.Quote{High: 1, Low: 0.1, Open: 0.9, Close: 0.8},
			expect: nil,
		},
		{
			q:      quoter.Quote{High: 1, Low: 0.1, Open: 0.9, Close: 0.2},
			expect: nil,
		},
		{
			q:      quoter.Quote{High: 1, Low: 0.1, Open: 0.4, Close: 0.5},
			expect: &Pattern{Name: StarBar, Sentiment: Neutral},
		},
	}
	for i, d := range data {
		p := starBar(d.q)
		if !reflect.DeepEqual(d.expect, p) {
			t.Fatalf("Test %d Expect: %#v, got %#v", i, d.expect, p)
		}
	}
}
