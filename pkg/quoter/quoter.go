package quoter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"
)

var client = http.Client{
	Timeout: 5 * time.Second,
}

type Quote struct {
	Symbol string
	High   float64
	Low    float64
	Open   float64
	Close  float64
}

func GetQuote(symbol string) (*Quote, error) {
	return rbfrx(symbol)
}

type rbfrxQuote struct {
	Status int
	OHLC   []rbfrxOHLC
}

type rbfrxOHLC struct {
	L float64 `json:"l"`
	H float64 `json:"h"`
	S float64 `json:"s"`
	E float64 `json:"e"`
}

func rbfrx(symbol string) (*Quote, error) {
	t := time.Now().UTC()
	day := t.YearDay() - 1
	callback := "jsonp" + strconv.FormatInt(t.Unix(), 10)
	//"https://price.roboforex.com/prime/2021/GBPUSD/D1/b?jsonp=jsonp1&from=111&to=111"
	URL := fmt.Sprintf(
		"https://price.roboforex.com/prime/%d/%s/D1/b?jsonp=%s&from=%d&to=%d",
		t.Year(),
		strings.ToUpper(symbol),
		callback,
		day,
		day,
	)
	rqst, err := http.NewRequest(http.MethodGet, URL, nil)
	if err != nil {
		return nil, fmt.Errorf("Can't create request: %q. %w", URL, err)
	}
	rqst.Header.Add("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/90.0.4430.72 Safari/537.36")
	resp, err := client.Do(rqst)
	if err != nil {
		return nil, fmt.Errorf("Can't send request: %q. %w", URL, err)
	}
	defer resp.Body.Close()
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Can't read body: %q. %w", URL, err)
	}
	b = bytes.TrimSpace(b)
	b = bytes.TrimPrefix(b, []byte(callback+"("))
	b = bytes.TrimSuffix(b, []byte(");"))
	var rq rbfrxQuote
	if err := json.Unmarshal(b, &rq); err != nil {
		return nil, fmt.Errorf("Can't unmarshal quote: %q. %w", b, err)
	}
	if rq.Status != 200 {
		return nil, fmt.Errorf("Response is not ok: %q. %w", b, err)
	}
	if len(rq.OHLC) == 0 {
		return nil, fmt.Errorf("No quotes: %q. %w", b, err)
	}
	q := Quote{
		Symbol: symbol,
		Open:   rq.OHLC[0].S,
		Close:  rq.OHLC[0].E,
		High:   rq.OHLC[0].H,
		Low:    rq.OHLC[0].L,
	}

	return &q, nil
}
