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

type Holder struct {
	m         sync.RWMutex
	db        map[string]*Quote
	lasUpdate time.Time
}

func NewHolder(symbols []string) *Holder {
	h := Holder{
		db: map[string]*Quote{},
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
				wRes.q.Symbol = strings.ToUpper(wRes.q.Symbol)
				h.db[wRes.q.Symbol] = &wRes.q
				log.Printf("Gor quote: %v", wRes.q)
			} else {
				log.Println("[ERROR] Can't fetch quote: %q. %v", wRes.q.Symbol, wRes.err)
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
func (h *Holder) GetQuote(symbol string) (*Quote, error) {
	h.m.RLock()
	defer h.m.RUnlock()
	symbol = strings.ToUpper(symbol)
	q, exist := h.db[symbol]
	if !exist {
		return nil, ErrNotAllowed
	}
	if q == nil {
		return nil, ErrNoQuote
	}
	rq := *q

	return &rq, nil
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
