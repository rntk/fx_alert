package controllers

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"sort"
	"strings"

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
		return processAddValue(dbH, qHolder, msg, *cmd)
	}

	if cmd.Command == commands.DeleteValue {
		return processDelete(dbH, msg, *cmd)
	}

	if cmd.Command == commands.ListValues {
		return processListValues(dbH, qHolder, msg, *cmd)
	}

	if cmd.Command == commands.DeltaValue {
		return processAddDeltaValues(dbH, qHolder, msg, *cmd)
	}

	return commands.HelpAnswer(), nil
}

func ProcessBotCommands(ctx context.Context, dbH *db.DB, qHolder *quoter.Holder, tlg *telegram.Telegram) {
	log.Printf("Bot commands controller started")
	for {
		select {
		case <-ctx.Done():
			return
		default:
			break
		}
		log.Println("[DEBUG] Telegram: wait updates")
		upds, err := tlg.GetUpdates(ctx, true)
		log.Println("[DEBUG] Telegram: got updates")
		if err != nil {
			log.Printf("Can't get update: %v", err)
			continue
		}
		for _, upd := range upds {
			msg := upd.Message
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

func processDelete(dbH *db.DB, msg telegram.Message, cmd commands.CommandValue) (*telegram.Answer, error) {
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
	if cmd.Value.Value == commands.NoValue {
		lst := dbH.List(msg.Chat.ID)
		k := strings.ToUpper(cmd.Value.Key)
		var deleted string
		for _, v := range lst {
			if !strings.Contains(strings.ToUpper(v.Key), k) {
				continue
			}
			if err := dbH.DeleteKey(msg.Chat.ID, v.Key); err != nil {
				deleted += "\n Can't delete: " + v.String()
				continue
			}
			deleted += "\n" + v.String()
		}

		return &telegram.Answer{Text: "Deleted: " + msg.Text + "\n" + deleted}, nil
	}
	if err := dbH.DeleteValue(msg.Chat.ID, *cmd.Value); err != nil {
		return nil, fmt.Errorf("Can't delete value: %w", err)
	}

	return &telegram.Answer{Text: "Deleted: " + msg.Text}, nil
}

func processAddDeltaValues(dbH *db.DB, qHolder *quoter.Holder, msg telegram.Message, cmd commands.CommandValue) (*telegram.Answer, error) {
	if cmd.Value == nil {
		return nil, errors.New("No delta value")
	}
	if cmd.Value.Value <= 0 {
		return nil, errors.New("Delta must be > 0")
	}
	symbols := quoter.GetAllowedSymbols()
	var answer string
	var levels []db.Value
	for _, symb := range symbols {
		if (cmd.Value.Key != "") && !strings.EqualFold(cmd.Value.Key, symb) {
			continue
		}
		d := cmd.Value.Value
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
	if len(levels) > 0 {
		if err := dbH.Add(msg.Chat.ID, levels); err != nil {
			msg := fmt.Sprintf("Can't save db: %v\n", err)
			log.Printf("[ERROR] %s", msg)
			answer += msg
		}
		answer = "Added levels: " + msg.Text + "\n" + answer
	} else {
		answer = "Symbol is not allowed: " + msg.Text
	}

	return &telegram.Answer{Text: answer}, nil
}

func processListValues(dbH *db.DB, qHolder *quoter.Holder, msg telegram.Message, cmd commands.CommandValue) (*telegram.Answer, error) {
	vals := dbH.List(msg.Chat.ID)
	if len(vals) == 0 {
		return &telegram.Answer{Text: "No alerts"}, nil
	}
	answer := ""
	sort.Slice(vals, func(i, j int) bool {
		return vals[i].Key <= vals[j].Key
	})
	var filter string
	if cmd.Value != nil {
		filter = strings.ToUpper(cmd.Value.Key)
	}
	for _, v := range vals {
		if (filter != "") && !strings.Contains(v.Key, filter) {
			continue
		}
		curr := 0.0
		if q, err := qHolder.GetQuote(v.Key); err == nil {
			curr = q.Close
		}
		answer += fmt.Sprintf("%s (%.5f) \n", v.String(), curr)
	}

	return &telegram.Answer{Text: answer}, nil
}

func processAddValue(dbH *db.DB, qHolder *quoter.Holder, msg telegram.Message, cmd commands.CommandValue) (*telegram.Answer, error) {
	if !quoter.IsValidSymbol(cmd.Value.Key) {
		return nil, errors.New("Invalid symbol")
	}
	if err := dbH.Add(msg.Chat.ID, []db.Value{*cmd.Value}); err != nil {
		return nil, fmt.Errorf("Can't add value: %w", err)
	}
	diff := ""
	q, err := qHolder.GetQuote(cmd.Value.Key)
	if err != nil {
		log.Printf("Can't get diff for: %q. %v", cmd.Value.Key, err)
	}
	if err == nil {
		diff = fmt.Sprintf(
			"Diff: %.5f \nCurrent: %.5f",
			math.Abs(q.Close-cmd.Value.Value),
			q.Close,
		)
	}

	return &telegram.Answer{Text: fmt.Sprintf("Added: %s \n%s", msg.Text, diff)}, nil
}
