package telegram

import (
	"github.com/Syfaro/telegram-bot-api"
	"log"
	"strings"
)

type Callback func(event string, args []string, chat int)

type Notifier struct {
	api  *tgbotapi.BotAPI
	chat int
	callbacks map[string][]Callback
}

func (n *Notifier) Handle(event string, callback Callback) {
	cbs := n.callbacks[event]
	cbs = append(cbs, callback)
	n.callbacks[event] = cbs
}

func (n *Notifier) handle(event string, args []string, chat int) {
	log.Println("notifier:", event, "args:", args, "chat:", chat)
	callbacks, ok := n.callbacks[event]
	if !ok {
		return
	}
	for _, callback := range callbacks {
		callback(event, args, chat)
	}
}

func (n *Notifier) updateLoop() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	err := n.api.UpdatesChan(u)
	if err != nil {
		log.Panic(err)
	}

	for update := range n.api.Updates {
		log.Printf("[%s] %s", update.Message.From.UserName, update.Message.Text)

		if !strings.HasPrefix(update.Message.Text, "/") {
			continue
		}
		args := strings.Split(update.Message.Text, " ")
		event := args[0]
		log.Println("handling event", args)
		if len(args) > 1 {
			n.handle(event, args[1:], update.Message.Chat.ID)
		} else {
			n.handle(event, nil, update.Message.Chat.ID)
		}
	}
}

func New(token string, chat int) Notifier {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Println("unable to create bot:", err, token)
		log.Print(err)
		panic(err)
	}

	notifier := Notifier{api: bot, chat: chat}
	notifier.callbacks = make(map[string][]Callback)
	go notifier.updateLoop()
	return notifier
}

func (n Notifier) Notify(message string) (err error) {
	msg := tgbotapi.NewMessage(n.chat, message)
	_, err = n.api.SendMessage(msg)
	return err
}
