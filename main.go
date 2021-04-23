package main

import (
	"flag"
	"fx_alert/pkg/controllers"
	"log"
	"os"
	"os/signal"
	"sync"

	"fx_alert/pkg/db"
	"fx_alert/pkg/telegram"
)

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
	stopBotCh := make(chan bool)
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		controllers.ProcessBotCommands(dbH, tlg, stopBotCh)
	}()
	stopQuotesCh := make(chan bool)
	wg.Add(1)
	go func() {
		defer wg.Done()
		controllers.ProcessQuotes(dbH, tlg, stopQuotesCh)
	}()
	stopCh := make(chan os.Signal)
	signal.Notify(stopCh, os.Kill, os.Interrupt)
	<-stopCh
	log.Print("Stopping...")
	stopBotCh <- true
	stopQuotesCh <- true
	wg.Wait()
	log.Print("Done")
}
