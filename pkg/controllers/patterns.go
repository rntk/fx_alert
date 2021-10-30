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

func ProcessPatterns(ctx context.Context, dbH *db.DB, qHolder *quoter.Holder, tlg *telegram.Telegram) {
	log.Printf("Patterns controller started")
	ticker := time.NewTicker(35 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			users := dbH.Users()
			if len(users) == 0 {
				continue
			}
			t := time.Now()
			if !isNewH1Bar(t) {
				continue
			}
			symbols := quoter.GetAllowedSymbols()
			var msgs []string
			hour := quoter.PreviousHour(t)
			for _, sym := range symbols {
				q, err := qHolder.GetQuoteByHour(sym, hour)
				if err != nil {
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
			answer := telegram.Answer{Text: strings.Join(msgs, "\n")}
			for _, ID := range users {
				if err := tlg.SendMessage(ID, 0, answer); err != nil {
					log.Printf("[ERROR] Can't send pattern to %d. %v. %s", ID, err, answer.Text)
				}
				log.Printf("[INFO] Patterns sent %d. %s", ID, answer.Text)
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
