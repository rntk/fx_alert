package quoter

import (
	"context"
	"log"
	"math/rand"
	"strings"
	"sync"
	"time"
)

type workerRes struct {
	q   Quote
	err error
}

type Quotes struct {
	Previous Quote
	Current  Quote
}

type Holder struct {
	m         sync.RWMutex
	db        map[string]*Quotes
	series    map[string]map[int]Quote
	lasUpdate time.Time
}

func NewHolder(symbols []string) *Holder {
	h := Holder{
		db:     map[string]*Quotes{},
		series: map[string]map[int]Quote{},
	}
	for _, symb := range symbols {
		symb = strings.ToUpper(symb)
		h.db[symb] = nil
	}

	return &h
}

// Update update quotes in storage.
// TODO: return error?
func (h *Holder) Update(ctx context.Context, workers uint) {
	if workers == 0 {
		return
	}
	h.m.Lock()
	defer h.m.Unlock()
	if t := time.Since(h.lasUpdate); t.Minutes() < 1 {
		log.Print("[INFO] Skip update less than 1 minute")
		return
	}
	symbCh := make(chan string, len(h.db))
	quCh := make(chan workerRes)
	for i := uint(0); i < workers; i++ {
		go worker(ctx, symbCh, quCh)
	}
	sendSymbN := 0
	for symb := range h.db {
		sendSymbN++
		symbCh <- symb
	}
	close(symbCh)
	recvQuN := 0
	for {
		select {
		case wRes := <-quCh:
			recvQuN++
			if wRes.err == nil {
				if h.db == nil {
					h.db = map[string]*Quotes{}
				}
				if h.series == nil {
					h.series = map[string]map[int]Quote{}
				}
				wRes.q.Symbol = strings.ToUpper(wRes.q.Symbol)
				qs := h.db[wRes.q.Symbol]
				if qs == nil {
					qs = &Quotes{
						Previous: wRes.q,
						Current:  wRes.q,
					}
				} else {
					qs.Previous = qs.Current
					qs.Current = wRes.q
				}
				hour := CurrentHour(time.Now())
				if _, exist := h.series[wRes.q.Symbol]; !exist {
					h.series[wRes.q.Symbol] = map[int]Quote{}
				}
				if q, exist := h.series[wRes.q.Symbol][hour]; exist {
					q.Close = wRes.q.Close
					h.series[wRes.q.Symbol][hour] = q
				} else {
					h.series[wRes.q.Symbol][hour] = wRes.q
				}
				h.db[wRes.q.Symbol] = qs
				log.Printf("Gor quote: %v", wRes.q)
			} else {
				log.Printf("[ERROR] Can't fetch quote: %q. %v", wRes.q.Symbol, wRes.err)
			}
			if recvQuN >= sendSymbN {
				h.lasUpdate = time.Now()
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

// GetQuote return quote by symbol.
func (h *Holder) GetQuote(symbol string) (*Quotes, error) {
	h.m.RLock()
	defer h.m.RUnlock()
	symbol = strings.ToUpper(symbol)
	qs, exist := h.db[symbol]
	if !exist {
		return nil, ErrNotAllowed
	}
	if qs == nil {
		return nil, ErrNoQuote
	}
	rq := *qs

	return &rq, nil
}

func (h *Holder) GetQuoteByHour(symbol string, hour int) (*Quote, error) {
	h.m.RLock()
	defer h.m.RUnlock()
	if _, exist := h.series[symbol]; !exist {
		return nil, ErrNoQuote
	}
	q, exist := h.series[symbol][hour]
	if !exist {
		return nil, ErrNoQuote
	}

	return &q, nil
}

// GetCurrentQuote return current quote.
func (h *Holder) GetCurrentQuote(symbol string) (*Quote, error) {
	qs, err := h.GetQuote(symbol)
	if err != nil {
		return nil, err
	}

	return &qs.Current, nil
}

// GetPreviousQuote return previouse quote.
func (h *Holder) GetPreviousQuote(symbol string) (*Quote, error) {
	qs, err := h.GetQuote(symbol)
	if err != nil {
		return nil, err
	}

	return &qs.Previous, nil
}

func worker(ctx context.Context, symbCh <-chan string, resultCh chan<- workerRes) {
	for {
		select {
		case symb, ok := <-symbCh:
			if !ok {
				return
			}
			q, err := rbfrx(symb)
			wr := workerRes{err: err}
			if err == nil {
				wr.q = *q
			} else {
				wr.q.Symbol = symb
			}
			resultCh <- wr
			time.Sleep(time.Duration(rand.Int31n(3)) * time.Second)
		case <-ctx.Done():
			return
		}
	}
}

func GetPrecision(symbol string) uint8 {
	symbol = strings.ToUpper(symbol)
	if strings.Contains(symbol, "JPY") {
		return 3
	}
	if strings.Contains(symbol, "BTC") {
		return 2
	}

	return 5
}

func ToPoints(symbol string, diff float64) int64 {
	if strings.EqualFold(symbol, "btcusd") {
		return int64(diff)
	}
	prec := GetPrecision(symbol)

	for i := 0; i < int(prec); i++ {
		diff *= 10
	}

	return int64(diff)
}

func FromPoints(symbol string, points int64) float64 {
	if strings.EqualFold(symbol, "btcusd") {
		return float64(points)
	}
	prec := GetPrecision(symbol)
	p := float64(points)
	for i := 0; i < int(prec); i++ {
		p /= 10
	}

	return p
}

func CurrentHour(t time.Time) int {
	return t.UTC().Hour()
}

func PreviousHour(t time.Time) int {
	h := CurrentHour(t)
	h -= 1
	if h < 0 {
		h = 23
	}

	return h
}
