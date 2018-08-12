package main

import (
	"os"

	"github.com/sirupsen/logrus"
)

func main() {
	botToken := os.Getenv("BOT_TOKEN")
	botChannel := os.Getenv("BOT_CHANNEL")
	botDBFile := os.Getenv("BOT_DB_FILE")
	iftttTriggerName := os.Getenv("IFTTT_TRIGGER_NAME")
	iftttTriggerKey := os.Getenv("IFTTT_TRIGGER_KEY")

	srv := NewBotServer(
		botToken, botChannel, botDBFile,
		iftttTriggerName, iftttTriggerKey,
	)
	if err := srv.Start(); err != nil {
		logrus.Fatal(err)
	}
}
