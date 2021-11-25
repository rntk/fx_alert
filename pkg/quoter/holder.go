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
	day int
}

type Quotes struct {
	Previous Quote
	Current  Quote
}

type Holder struct {
	m          sync.RWMutex
	db         map[string]*Quotes
	seriesHour map[string]map[int]Quote
	seriesDay  map[string]map[int]Quote
	lasUpdate  time.Time
	prevDay    int
}

func NewHolder(symbols []string) *Holder {
	h := Holder{
		db:         map[string]*Quotes{},
		seriesHour: map[string]map[int]Quote{},
		seriesDay:  map[string]map[int]Quote{},
		prevDay:    -1,
	}
	for _, symb := range symbols {
		symb = strings.ToUpper(symb)
		h.db[symb] = nil
	}

	return &h
}

type symbolToFetch struct {
	Day    int
	Symbol string
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
	t := time.Now()
	currentDay := t.YearDay()
	symbChSize := len(h.db)
	if currentDay != h.prevDay {
		symbChSize *= 2
	}
	symbCh := make(chan symbolToFetch, symbChSize)
	quCh := make(chan workerRes)
	for i := uint(0); i < workers; i++ {
		go worker(ctx, symbCh, quCh)
	}
	sendSymbN := 0
	for symb := range h.db {
		sendSymbN++
		symbCh <- symbolToFetch{
			Symbol: symb,
			Day:    currentDay,
		}
	}
	if currentDay != h.prevDay {
		for symb := range h.db {
			sendSymbN++
			symbCh <- symbolToFetch{
				Symbol: symb,
				Day:    PreviousDay(t),
			}
		}
	}
	close(symbCh)
	recvQuN := 0
	for {
		select {
		case wRes, ok := <-quCh:
			if !ok {
				return
			}
			recvQuN++
			if wRes.err == nil {
				if currentDay == wRes.day {
					h.saveCurrentDayQuotes(wRes.q)
				} else {
					h.saveDayQuotes(wRes.q, wRes.day)
				}
				log.Printf("Got quote: %v", wRes.q)
			} else {
				log.Printf("[ERROR] Can't fetch quote: %q. %v", wRes.q.Symbol, wRes.err)
			}
			if recvQuN >= sendSymbN {
				h.lasUpdate = time.Now()
				return
			}
			h.prevDay = currentDay
		case <-ctx.Done():
			return
		}
	}
}

func (h *Holder) saveCurrentDayQuotes(q Quote) {
	if h.db == nil {
		h.db = map[string]*Quotes{}
	}
	if h.seriesHour == nil {
		h.seriesHour = map[string]map[int]Quote{}
	}
	q.Symbol = strings.ToUpper(q.Symbol)
	qs := h.db[q.Symbol]
	if qs == nil {
		qs = &Quotes{
			Previous: q,
			Current:  q,
		}
	} else {
		qs.Previous = qs.Current
		qs.Current = q
	}
	t := time.Now()
	hour := CurrentHour(t)
	if h.seriesHour[q.Symbol] == nil {
		h.seriesHour[q.Symbol] = map[int]Quote{}
	}
	qq, exist := h.seriesHour[q.Symbol][hour]
	if exist {
		if qq.High < q.Close {
			qq.High = q.Close
		}
		if qq.Low > q.Close {
			qq.Low = q.Close
		}
		qq.Close = q.Close
	} else {
		qq = Quote{
			Symbol: q.Symbol,
			High:   q.Close,
			Low:    q.Close,
			Open:   q.Close,
			Close:  q.Close,
		}
	}
	if h.seriesHour[q.Symbol] == nil {
		h.seriesHour[q.Symbol] = map[int]Quote{}
	}
	h.seriesHour[q.Symbol][hour] = qq
	h.db[q.Symbol] = qs
}

func (h *Holder) saveDayQuotes(q Quote, day int) {
	q.Symbol = strings.ToUpper(q.Symbol)
	if h.seriesDay == nil {
		h.seriesDay = map[string]map[int]Quote{}
	}
	if h.seriesDay[q.Symbol] == nil {
		h.seriesDay[q.Symbol] = map[int]Quote{}
	}
	h.seriesDay[q.Symbol][day] = q
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
	if _, exist := h.seriesHour[symbol]; !exist {
		return nil, ErrNoQuote
	}
	q, exist := h.seriesHour[symbol][hour]
	if !exist {
		return nil, ErrNoQuote
	}

	return &q, nil
}

func (h *Holder) GetQuoteByDay(symbol string, day int) (*Quote, error) {
	h.m.RLock()
	defer h.m.RUnlock()
	if _, exist := h.seriesDay[symbol]; !exist {
		return nil, ErrNoQuote
	}
	q, exist := h.seriesDay[symbol][day]
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

func worker(ctx context.Context, symbCh <-chan symbolToFetch, resultCh chan<- workerRes) {
	for {
		select {
		case symb, ok := <-symbCh:
			if !ok {
				return
			}
			q, err := rbfrx(symb.Symbol, symb.Day)
			wr := workerRes{
				err: err,
				day: symb.Day,
			}
			if err == nil {
				wr.q = *q
			} else {
				wr.q.Symbol = symb.Symbol
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

func CurrentDay(t time.Time) int {
	return t.YearDay()
}

func PreviousDay(t time.Time) int {
	d := CurrentDay(t)
	d -= 1
	if d <= 0 {
		d = 365
		if t.Year()%4 == 0 { // leap year
			d += 1
		}
	}

	return d
}
