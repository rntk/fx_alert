package main

import (
	"flag"
	"fmt"
	"log"
	"os"
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
	} else if cmd.Command == commands.DeleteValue {
		if err := dbH.DeleteValue(msg.Chat.ID, cmd.Value.Key, cmd.Value.Value); err != nil {
			return "", fmt.Errorf("Can't delete value: %w", err)
		}

		return "Deleted: " + msg.Text, nil
	}
	if cmd.Command == commands.ListValues {
		answer := ""
		vals := dbH.List(msg.Chat.ID)
		for _, v := range vals {
			answer += fmt.Sprintf("%s %s %.5f\n", v.Key, v.Type, v.Value)
		}
		if answer == "" {
			answer = "No alerts"
		}

		return answer, nil
	}
	answer := fmt.Sprintf(
		`
Add: %s EURUSD %s 1.2550
Delete: %s EURUSD %s 1.2550
List: %s
Help: %s
`,
		commands.AddValue,
		db.AboveCurrent,
		commands.DeleteValue,
		db.BelowCurrent,
		commands.ListValues,
		commands.Help,
	)

	return answer, nil
}

func main() {
	dbPath := flag.String("db", "db.json", "path to database")
	dbH, err := db.New(*dbPath, true)
	if err != nil {
		log.Panicf("Can't create database: %q. %v", dbPath, err)
	}

	token := os.Getenv("BOT_TOKEN")
	if token == "" {
		log.Panicf("BOT_TOKEN not set")
	}
	tlg := telegram.New(token)
	defer tlg.Stop()
	msgCh := tlg.Start(3 * time.Second)
	errCh := tlg.Errors()
	for {
		select {
		case err := <-errCh:
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
