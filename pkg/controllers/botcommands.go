package controllers

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sort"
	"time"

	"fx_alert/pkg/commands"
	"fx_alert/pkg/db"
	"fx_alert/pkg/quoter"
	"fx_alert/pkg/telegram"
)

func processCommand(dbH *db.DB, qHolder *quoter.Holder, msg telegram.Message) (*telegram.Answer, error) {
	cmd, err := commands.Parse(msg.Text)
	if err != nil {
		return nil, fmt.Errorf("Can't parse command: %w", err)
	}

	if cmd.Command == commands.AddValue {
		if err := dbH.Add(msg.Chat.ID, []db.Value{*cmd.Value}); err != nil {
			return nil, fmt.Errorf("Can't add value: %w", err)
		}

		return &telegram.Answer{Text: "Added: " + msg.Text}, nil
	}

	if cmd.Command == commands.DeleteValue {
		return processDeleteCommand(dbH, msg, *cmd)
	}

	if cmd.Command == commands.ListValues {
		vals := dbH.List(msg.Chat.ID)
		if len(vals) == 0 {
			return &telegram.Answer{Text: "No alerts"}, nil
		}
		answer := ""
		sort.Slice(vals, func(i, j int) bool {
			return vals[i].Key <= vals[j].Key
		})
		for _, v := range vals {
			curr := 0.0
			if q, err := qHolder.GetQuote(v.Key); err == nil {
				curr = q.Close
			}
			answer += fmt.Sprintf("%s (%.5f) \n", v.String(), curr)
		}

		return &telegram.Answer{Text: answer}, nil
	}

	if cmd.Command == commands.DeltaValue {
		return processAddDeltaValues(dbH, qHolder, msg, *cmd)
	}

	return commands.HelpAnswer(), nil
}

func ProcessBotCommands(ctx context.Context, dbH *db.DB, qHolder *quoter.Holder, tlg *telegram.Telegram) {
	msgCh := tlg.Start(10 * time.Second)
	errCh := tlg.Errors()
	log.Printf("Bot commands controller started")
	for {
		select {
		case <-ctx.Done():
			return
		case err := <-errCh:
			log.Printf("ERROR: %v", err)
		case msg := <-msgCh:
			log.Printf("Got message: %v", msg)
			answer, err := processCommand(dbH, qHolder, msg)
			if err != nil {
				answer = &telegram.Answer{Text: "Can't process command"}
				log.Printf("Can't process command: %q. %v", msg.Text, err)
			}
			if err := tlg.SendMessage(msg.Chat.ID, msg.MessageID, *answer); err != nil {
				log.Printf("Can't send message: %v. %v", answer, err)
			}
		}
	}
}

func processDeleteCommand(dbH *db.DB, msg telegram.Message, cmd commands.CommandValue) (*telegram.Answer, error) {
	if cmd.Value == nil {
		vals := dbH.List(msg.Chat.ID)
		if len(vals) == 0 {
			return &telegram.Answer{Text: "No alerts"}, nil
		}
		var btns [][]telegram.KeyboardButton
		for _, v := range vals {
			btns = append(
				btns,
				[]telegram.KeyboardButton{
					{Text: fmt.Sprintf("%s %s", commands.DeleteValue, v.String())},
				},
			)
		}
		rk := &telegram.ReplyKeyboardMarkup{
			Keyboard:        btns,
			OneTimeKeyboard: true,
		}

		return &telegram.Answer{Text: "Select: ", ReplyKeyboard: rk}, nil
	}
	if err := dbH.DeleteValue(msg.Chat.ID, *cmd.Value); err != nil {
		return nil, fmt.Errorf("Can't delete value: %w", err)
	}

	return &telegram.Answer{Text: "Deleted: " + msg.Text}, nil
}

func processAddDeltaValues(dbH *db.DB, qHolder *quoter.Holder, msg telegram.Message, cmd commands.CommandValue) (*telegram.Answer, error) {
	if cmd.Delta <= 0 {
		return nil, errors.New("Delta must be > 0")
	}
	symbols := quoter.GetAllowedSymbols()
	var answer string
	var levels []db.Value
	for _, symb := range symbols {
		d := float64(cmd.Delta)
		prec := quoter.GetPrecision(symb)
		q, err := qHolder.GetQuote(symb)
		if err != nil {
			msg := fmt.Sprintf("Can`t add delta for: %q. %v\n", symb, err)
			answer += msg
			log.Printf("[ERROR] Can't add delta value %s", msg)
			continue
		}
		for i := 0; i < int(prec); i++ {
			d /= 10
		}
		levels = append(levels, db.Value{
			Key:       symb,
			Value:     q.Close + d,
			Precision: prec,
			Type:      db.BelowCurrent,
		})
		levels = append(levels, db.Value{
			Key:       symb,
			Value:     q.Close - d,
			Precision: prec,
			Type:      db.AboveCurrent,
		})
	}
	if err := dbH.Add(msg.Chat.ID, levels); err != nil {
		msg := fmt.Sprintf("Can't save db: %v\n", err)
		log.Printf("[ERROR] %s", msg)
		answer += msg
	}
	answer = "Added levels: " + msg.Text + "\n" + answer

	return &telegram.Answer{Text: answer}, nil
}
