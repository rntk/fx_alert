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

	if cmd.Command == commands.AddLevelValue {
		if err := dbH.AddLevel(msg.Chat.ID, *cmd.Level); err != nil {
			return nil, fmt.Errorf("Can't add value: %w", err)
		}

		return &telegram.Answer{Text: "Added: " + msg.Text}, nil
	}
	if cmd.Command == commands.AddDeltaValue {
		var dVals []db.DeltaValue
		if cmd.Delta.Key == commands.DeltaAllSymbols {
			symbols := quoter.GetAllowedSymbols()
			for _, symb := range symbols {
				q, err := qHolder.GetQuote(symb)
				if err != nil {
					return nil, fmt.Errorf("Can't get quote for: %q. %w", symb, err)
				}
				dv := *cmd.Delta
				dv.Key = symb
				dv.Value = q.Close
				dVals = append(dVals, dv)
			}
		} else {
			dVals = append(dVals, *cmd.Delta)
		}
		if err := dbH.AddDeltas(msg.Chat.ID, dVals); err != nil {
			return nil, fmt.Errorf("Can't add value: %w", err)
		}

		return &telegram.Answer{Text: "Added: " + msg.Text}, nil
	}

	if cmd.Command == commands.DeleteLevelValue {
		return processDeleteLevelCommand(dbH, msg, *cmd)
	}
	if cmd.Command == commands.DeleteDeltaValue {
		return processDeleteDeltaCommand(dbH, msg, *cmd)
	}

	if cmd.Command == commands.ListLevelValues {
		vals := dbH.ListLevels(msg.Chat.ID)
		if len(vals) == 0 {
			return &telegram.Answer{Text: "No alerts"}, nil
		}
		answer := ""
		sort.Slice(vals, func(i, j int) bool {
			return vals[i].Key <= vals[j].Key
		})
		for _, v := range vals {
			answer += v.String() + "\n"
		}

		return &telegram.Answer{Text: answer}, nil
	}
	if cmd.Command == commands.ListDeltaValues {
		vals := dbH.ListDeltas(msg.Chat.ID)
		if len(vals) == 0 {
			return &telegram.Answer{Text: "No alerts"}, nil
		}
		answer := ""
		sort.Slice(vals, func(i, j int) bool {
			return vals[i].Key <= vals[j].Key
		})
		for _, v := range vals {
			answer += v.String() + "\n"
		}

		return &telegram.Answer{Text: answer}, nil
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
				if errors.Is(err, quoter.ErrNoQuote) {
					answer = &telegram.Answer{Text: "Not all quotes are available. Please try later"}
				} else {
					answer = &telegram.Answer{Text: "Can't process command"}
				}
				log.Printf("Can't process command: %q. %v", msg.Text, err)
			}
			if err := tlg.SendMessage(msg.Chat.ID, msg.MessageID, *answer); err != nil {
				log.Printf("Can't send message: %v. %v", answer, err)
			}
		}
	}
}

func processDeleteLevelCommand(dbH *db.DB, msg telegram.Message, cmd commands.CommandValue) (*telegram.Answer, error) {
	if cmd.Level == nil {
		vals := dbH.ListLevels(msg.Chat.ID)
		if len(vals) == 0 {
			return &telegram.Answer{Text: "No alerts"}, nil
		}
		var btns [][]telegram.KeyboardButton
		for _, v := range vals {
			btns = append(
				btns,
				[]telegram.KeyboardButton{
					{Text: fmt.Sprintf("%s %s", commands.DeleteLevelValue, v.String())},
				},
			)
		}
		rk := &telegram.ReplyKeyboardMarkup{
			Keyboard:        btns,
			OneTimeKeyboard: true,
		}

		return &telegram.Answer{Text: "Select: ", ReplyKeyboard: rk}, nil
	}
	if err := dbH.DeleteLevelValue(msg.Chat.ID, cmd.Level.Key, cmd.Level.Value); err != nil {
		return nil, fmt.Errorf("Can't delete value: %w", err)
	}

	return &telegram.Answer{Text: "Deleted: " + msg.Text}, nil
}

func processDeleteDeltaCommand(dbH *db.DB, msg telegram.Message, cmd commands.CommandValue) (*telegram.Answer, error) {
	if cmd.Delta == nil {
		vals := dbH.ListDeltas(msg.Chat.ID)
		if len(vals) == 0 {
			return &telegram.Answer{Text: "No alerts"}, nil
		}
		var btns [][]telegram.KeyboardButton
		for _, v := range vals {
			btns = append(
				btns,
				[]telegram.KeyboardButton{
					{Text: fmt.Sprintf("%s %s", commands.DeleteDeltaValue, v.String())},
				},
			)
		}
		rk := &telegram.ReplyKeyboardMarkup{
			Keyboard:        btns,
			OneTimeKeyboard: true,
		}

		return &telegram.Answer{Text: "Select: ", ReplyKeyboard: rk}, nil
	}
	if err := dbH.DeleteDeltaValue(msg.Chat.ID, cmd.Delta.Key, cmd.Delta.Value); err != nil {
		return nil, fmt.Errorf("Can't delete value: %w", err)
	}

	return &telegram.Answer{Text: "Deleted: " + msg.Text}, nil
}
