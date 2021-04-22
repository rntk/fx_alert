package main

import (
	"log"
	"os"
	"time"

	"fx_alert/pkg/telegram"
)

func main() {
	token := os.Getenv("BOT_TOKEN")
	if token == "" {
		log.Fatalf("BOT_TOKEN not set")
	}
	tlg := telegram.New(token)
	defer tlg.Stop()
	msgCh := tlg.Start(10 * time.Second)
	errCh := tlg.Errors()
	for {
		select {
		case err := <- errCh:
			log.Printf("Error: %v", err)
			tlg.Stop()
			log.Panicf("error")
		case msg := <- msgCh:
			println(msg.Text)
		}
	}
}
