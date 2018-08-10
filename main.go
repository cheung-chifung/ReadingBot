package main

import (
	"os"

	"github.com/sirupsen/logrus"
)

func main() {
	botToken := os.Getenv("BOT_TOKEN")
	botChannel := os.Getenv("BOT_CHANNEL")
	botDBFile := os.Getenv("BOT_DB_FILE")

	srv := NewBotServer(botToken, botChannel, botDBFile)
	if err := srv.Start(); err != nil {
		logrus.Fatal(err)
	}
}
