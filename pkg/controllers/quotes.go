package controllers

import (
	"context"
	"fmt"
	"log"
	"math"
	"strings"
	"time"

	"fx_alert/pkg/db"
	"fx_alert/pkg/quoter"
	"fx_alert/pkg/telegram"
)

func ProcessQuotes(ctx context.Context, dbH *db.DB, qHolder *quoter.Holder, tlg *telegram.Telegram) {
	levelTicker := time.NewTicker(65 * time.Second)
	defer levelTicker.Stop()
	momentumTicker := time.NewTicker(5 * time.Minute)
	defer momentumTicker.Stop()
	log.Printf("Quotes controller started")
	go qHolder.Update(ctx, 2)
	for {
		select {
		case <-ctx.Done():
			return
		case <-levelTicker.C:
			qHolder.Update(ctx, 2)
			checkUsersLevelAlerts(ctx, dbH, qHolder, tlg)
		case <-momentumTicker.C:
			qHolder.Update(ctx, 2)
			checkMomentum(ctx, dbH, qHolder, tlg)
		}
	}
}

func checkUsersLevelAlerts(ctx context.Context, dbH *db.DB, qHolder *quoter.Holder, tlg *telegram.Telegram) {
	ids := dbH.Users()
	for _, ID := range ids {
		select {
		case <-ctx.Done():
			return
		default:
			break
		}
		values := dbH.List(ID)
		for _, val := range values {
			select {
			case <-ctx.Done():
				return
			default:
				break
			}
			q, err := qHolder.GetCurrentQuote(val.Key)
			if err != nil {
				log.Printf("Can't get quotes to check levels: %d. %q. %v", ID, val.Key, err)
				continue
			}
			if !val.IsAlert(q.Close) {
				continue
			}
			go func(ID int64, val db.Value, p float64) {
				msg := fmt.Sprintf("Alert: %s.  \t  Current: %.5f", val.String(), p)
				if err := tlg.SendMessage(ID, 0, telegram.Answer{Text: msg}); err != nil {
					log.Printf("Can't send alert: %d. %q. %v", ID, msg, err)
					return
				}
				log.Printf("Sent alert: %d. %q", ID, msg)
				if err := dbH.DeleteValue(ID, val); err != nil {
					log.Printf("Can't delete: %d. %q. %v", ID, val.String(), err)
					return
				}
				if val.DeltaID != "" {
					if err := ensureDeltaValues(dbH, qHolder, ID, val.Key); err != nil {
						log.Printf("Can't add delta values: %d - %s", ID, val.Key)
					}
				}
				log.Printf("Deleted: %d. %q", ID, val.String())
			}(ID, val, q.Close)
		}
	}
}

func ensureDeltaValues(dbH *db.DB, qHolder *quoter.Holder, ID int64, symb string) error {
	us, err := dbH.GetSettings(ID)
	if err != nil {
		return err
	}
	if us.Delta <= 0 {
		return nil
	}
	q, err := qHolder.GetCurrentQuote(symb)
	if err != nil {
		return err
	}
	prec := quoter.GetPrecision(symb)
	d := quoter.FromPoints(symb, int64(us.Delta))
	levels := []db.Value{
		{
			Key:       symb,
			Value:     q.Close + d,
			Precision: prec,
			Type:      db.BelowCurrent,
		},
		{
			Key:       symb,
			Value:     q.Close - d,
			Precision: prec,
			Type:      db.AboveCurrent,
		},
	}
	if err := dbH.Add(ID, levels); err != nil {
		return err
	}

	return nil
}

func checkMomentum(ctx context.Context, dbH *db.DB, qHolder *quoter.Holder, tlg *telegram.Telegram) {
	ids := dbH.Users()
	for _, ID := range ids {
		select {
		case <-ctx.Done():
			return
		default:
			break
		}
		symbs := quoter.GetAllowedSymbols()
		for _, symb := range symbs {
			select {
			case <-ctx.Done():
				return
			default:
				break
			}
			qs, err := qHolder.GetQuote(symb)
			if err != nil {
				log.Printf("Can't get quotes to check momentum: %d. %q. %v", ID, symb, err)
				continue
			}
			diff := qs.Current.Close - qs.Previous.Close
			points := quoter.ToPoints(symb, math.Abs(diff))
			if strings.EqualFold(symb, "btcusd") && (points < 500) {
				continue
			}
			if points < 50 {
				continue
			}
			go func(ID int64, symb string, diff float64, points int64, qs *quoter.Quotes) {
				msg := fmt.Sprintf(
					"Diff: %s - %d (%.5f)\tPrevious: %.5f\tCurrent: %.5f",
					symb,
					points,
					diff,
					qs.Previous.Close,
					qs.Current.Close,
				)
				if err := tlg.SendMessage(ID, 0, telegram.Answer{Text: msg}); err != nil {
					log.Printf("Can't send alert: %d. %q. %v", ID, msg, err)
					return
				}
				log.Printf("Sent alert: %d. %q", ID, msg)
			}(ID, symb, diff, points, qs)
		}
	}
}
