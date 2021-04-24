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

func processCommand(dbH *db.DB, msg telegram.Message) (string, error) {
	cmd, err := commands.Parse(msg.Text)
	if err != nil {
		return "", fmt.Errorf("Can't parse command: %w", err)
	}

	if cmd.Command == commands.AddValue {
		if err := dbH.Add(msg.Chat.ID, *cmd.Value); err != nil {
			return "", fmt.Errorf("Can't add value: %w", err)
		}

		return "Added: " + msg.Text, nil
	}

	if cmd.Command == commands.DeleteValue {
		if err := dbH.DeleteValue(msg.Chat.ID, cmd.Value.Key, cmd.Value.Value); err != nil {
			return "", fmt.Errorf("Can't delete value: %w", err)
		}

		return "Deleted: " + msg.Text, nil
	}

	if cmd.Command == commands.ListValues {
		answer := ""
		vals := dbH.List(msg.Chat.ID)
		for _, v := range vals {
			answer += v.String() + "\n"
		}
		if answer == "" {
			answer = "No alerts"
		}

		return answer, nil
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
			// TODO: do not panic, send message?
			log.Panicf("Error: %v", err)
		case msg := <-msgCh:
			log.Printf("Got message: %v", msg)
			answer, err := processCommand(dbH, msg)
			if err != nil {
				answer = "Can't process command"
				log.Printf("Can't process command: %q. %v", msg.Text, err)
			}
			if err := tlg.SendMessage(msg.Chat.ID, answer, msg.MessageID); err != nil {
				log.Printf("Can't send message: %q. %v", answer, err)
			}
		}
	}
}
