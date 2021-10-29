package patterns

import (
	"reflect"
	"testing"

	"fx_alert/pkg/quoter"
)

func TestPinBar(t *testing.T) {
	type tableData struct {
		q      quoter.Quote
		expect *Pattern
	}

	data := []tableData{
		{
			q:      quoter.Quote{High: 1, Low: 0.1, Open: 0.9, Close: 0.8},
			expect: &Pattern{Name: PinBar, Sentiment: Bull},
		},
		{
			q:      quoter.Quote{High: 1, Low: 0.1, Open: 0.2, Close: 0.3},
			expect: &Pattern{Name: PinBar, Sentiment: Bear},
		},
		{
			q:      quoter.Quote{High: 1, Low: 0.1, Open: 0.8, Close: 0.9},
			expect: &Pattern{Name: PinBar, Sentiment: Bull},
		},
		{
			q:      quoter.Quote{High: 1, Low: 0.1, Open: 0.3, Close: 0.2},
			expect: &Pattern{Name: PinBar, Sentiment: Bear},
		},
		{
			q:      quoter.Quote{High: 1, Low: 0.1, Open: 0.8, Close: 1},
			expect: &Pattern{Name: PinBar, Sentiment: Bull},
		},
		{
			q:      quoter.Quote{High: 1, Low: 0.1, Open: 0.3, Close: 0.1},
			expect: &Pattern{Name: PinBar, Sentiment: Bear},
		},
		{
			q:      quoter.Quote{High: 1, Low: 0.1, Open: 0.7, Close: 0.3},
			expect: nil,
		},
		{
			q:      quoter.Quote{High: 1, Low: 0.1, Open: 0.1, Close: 0.9},
			expect: nil,
		},
	}
	for i, d := range data {
		p := pinBar(d.q)
		if !reflect.DeepEqual(d.expect, p) {
			t.Fatalf("Test %d Expect: %#v, got %#v", i, d.expect, p)
		}
	}
}
