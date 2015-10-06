package main

import (
    "log"
    "github.com/Syfaro/telegram-bot-api"
	"fmt"
	"time"
)

func main() {
    bot, err := tgbotapi.NewBotAPI("")
    if err != nil {
        log.Panic(err)
    }

    bot.Debug = true

    log.Printf("Authorized on account %s", bot.Self.UserName)

    u := tgbotapi.NewUpdate(0)
    u.Timeout = 60

    err = bot.UpdatesChan(u)
    if err != nil {
        log.Panic(err)
    }

	go func() {
		ticker := time.NewTicker(time.Second * 3)
		for t := range ticker.C {
			msg := tgbotapi.NewMessage(1863832, t.String())
			bot.SendMessage(msg)
		}
	}()

    for update := range bot.Updates {
        log.Printf("[%s] %s", update.Message.From.UserName, update.Message.Text)

        msg := tgbotapi.NewMessage(update.Message.Chat.ID, update.Message.Text)
//        msg.ReplyToMessageID = update.Message.MessageID
		msg.Text = "Kek"

        bot.SendMessage(msg)

		msg = tgbotapi.NewMessage(update.Message.Chat.ID, update.Message.Text)
		//        msg.ReplyToMessageID = update.Message.MessageID
		msg.Text = "Pek"
		fmt.Println("chat id", update.Message.Chat.ID)

		bot.SendMessage(msg)
    }
}
