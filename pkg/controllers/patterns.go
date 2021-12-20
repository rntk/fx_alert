package controllers

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"fx_alert/pkg/db"
	"fx_alert/pkg/patterns"
	"fx_alert/pkg/quoter"
	"fx_alert/pkg/telegram"
)

const (
	timeframeDay  = "day"
	timeframeHour = "hour"
)

func ProcessPatterns(ctx context.Context, dbH *db.DB, qHolder *quoter.Holder, tlg *telegram.Telegram) {
	log.Printf("Patterns controller started")
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	checked := map[string]int{
		timeframeHour: -1,
		timeframeDay:  -1,
	}
	checkTime := map[string]func(time.Time) bool{
		timeframeHour: isNewH1Bar,
		timeframeDay:  func(t time.Time) bool { return true },
	}
	getQuotes := map[string]func(string, int) (*quoter.Quote, error){
		timeframeHour: qHolder.GetQuoteByHour,
		timeframeDay:  qHolder.GetQuoteByDay,
	}
	for {
		select {
		case <-ticker.C:
			users := dbH.Users()
			if len(users) == 0 {
				continue
			}
			t := time.Now()
			for tf := range checked {
				if !checkTime[tf](t) {
					continue
				}
				var timeToCheck int
				if tf == timeframeDay {
					tt := quoter.PreviousDay("btcusd", t)
					timeToCheck = tt.YearDay()
				} else {
					timeToCheck = quoter.PreviousHour(t)
				}
				if timeToCheck == checked[tf] {
					continue
				}
				checked[tf] = timeToCheck
				symbols := quoter.GetAllowedSymbols()
				var msgs []string
				for _, sym := range symbols {
					if tf == timeframeDay {
						tt := quoter.PreviousDay(sym, t)
						timeToCheck = tt.YearDay()
					}
					q, err := getQuotes[tf](sym, timeToCheck)
					if err != nil {
						if tf == timeframeDay {
							checked[tf] = -1
						}
						continue
					}
					p := patterns.FindPattern(*q)
					if p == nil {
						continue
					}
					msgs = append(
						msgs,
						fmt.Sprintf(
							"%s - %s (%s)",
							strings.ToUpper(sym),
							string(p.Name),
							string(p.Sentiment),
						),
					)
				}
				if len(msgs) == 0 {
					continue
				}
				answer := telegram.Answer{Text: tf + "\n" + strings.Join(msgs, "\n")}
				for _, ID := range users {
					if err := tlg.SendMessage(ID, 0, answer); err != nil {
						log.Printf("[ERROR] Can't send pattern to %d. %v. %s", ID, err, answer.Text)
					}
					log.Printf("[INFO] Patterns sent %d. %s", ID, answer.Text)
				}
			}
		case <-ctx.Done():
			return
		}
	}
}

func isNewH1Bar(t time.Time) bool {
	m := t.Minute()

	return m == 0
}
