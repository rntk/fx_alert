package controllers

import (
	"fmt"
	"log"
	"time"

	"fx_alert/pkg/db"
	"fx_alert/pkg/quoter"
	"fx_alert/pkg/telegram"
)

func ProcessQuotes(dbH *db.DB, tlg *telegram.Telegram, stopCh <-chan bool) {
	ticker := time.NewTicker(time.Minute)
	log.Printf("Quotes controller started")
	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			ids := dbH.Users()
			currentValue := map[string]float64{}
			for _, ID := range ids {
				select {
				case <-stopCh:
					return
				default:
					break
				}
				values := dbH.List(ID)
				for _, val := range values {
					select {
					case <-stopCh:
						return
					default:
						break
					}
					p, exists := currentValue[val.Key]
					if !exists {
						q, err := quoter.GetQuote(val.Key)
						if err != nil {
							log.Printf("Can't get quotes for: %q. %q. %v", ID, val.Key, err)
							continue
						}
						log.Printf("Got qoute: %v", q)
						currentValue[val.Key] = q.Close
						p = q.Close
					}
					if !val.IsAlert(p) {
						continue
					}
					go func(ID int64, val db.Value, p float64) {
						msg := fmt.Sprintf("Alert: %s. \n. Current: %.5f", val.String(), p)
						if err := tlg.SendMessage(ID, msg, 0); err != nil {
							log.Printf("Can't send alert: %d. %q. %v", ID, msg, err)
							return
						}
						log.Printf("Sent alert: %d. %q", ID, msg)
						if err := dbH.DeleteValue(ID, val.Key, val.Value); err != nil {
							log.Printf("Can't delete: %d. %q. %v", ID, val.String(), err)
							return
						}
						log.Printf("Deleted: %d. %q", ID, val.String())
					}(ID, val, p)
				}
			}
		}
	}
}