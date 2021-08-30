package controllers

import (
	"context"
	"fmt"
	"log"
	"time"

	"fx_alert/pkg/commands"
	"fx_alert/pkg/db"
	"fx_alert/pkg/telegram"
)

func processCommand(dbH *db.DB, msg telegram.Message) (*telegram.Answer, error) {
	cmd, err := commands.Parse(msg.Text)
	if err != nil {
		return nil, fmt.Errorf("Can't parse command: %w", err)
	}

	if cmd.Command == commands.AddValue {
		if err := dbH.Add(msg.Chat.ID, *cmd.Value); err != nil {
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
		for _, v := range vals {
			answer += v.String() + "\n"
		}

		return &telegram.Answer{Text: answer}, nil
	}

	return commands.HelpAnswer(), nil
}

func ProcessBotCommands(ctx context.Context, dbH *db.DB, tlg *telegram.Telegram) {
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
			answer, err := processCommand(dbH, msg)
			if err != nil {
				answer = &telegram.Answer{Text: "Can't process command"}
				log.Printf("Can't process command: %q. %v", msg.Text, err)
			}
			if err := tlg.SendMessage(msg.Chat.ID, msg.MessageID, *answer); err != nil {
				log.Printf("Can't send message: %q. %v", answer, err)
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
	if err := dbH.DeleteValue(msg.Chat.ID, cmd.Value.Key, cmd.Value.Value); err != nil {
		return nil, fmt.Errorf("Can't delete value: %w", err)
	}

	return &telegram.Answer{Text: "Deleted: " + msg.Text}, nil
}
