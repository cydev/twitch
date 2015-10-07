package telegram

import (
	"github.com/Syfaro/telegram-bot-api"
	"log"
)

type Notifier struct {
	api  *tgbotapi.BotAPI
	chat int
}

func New(token string, chat int) Notifier {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Println("unable to create bot:", err, token)
		log.Print(err)
		panic(err)
	}
	return Notifier{api: bot, chat: chat}
}

func (n Notifier) Notify(message string) (err error) {
	msg := tgbotapi.NewMessage(n.chat, message)
	_, err = n.api.SendMessage(msg)
	return err
}
