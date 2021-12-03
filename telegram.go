package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	tb "gopkg.in/tucnak/telebot.v2"
)

type COMMAND int

const (
	CommandGetLink = iota
	CommandAddWhiteList
	CommandCheckWhiteList
	CommandCheckUser
)

type cdata struct {
	UserID int
}

type callback func(command COMMAND, payload cdata) (string, error)

type TgBot struct {
	tg *tb.Bot

	token string
	host  string
	cb    callback
	group string
	owner string

	waitingID bool
}

func NewTgBot(token string, group int, owner int, host string, cb callback) (*TgBot, error) {
	var err error

	var poller tb.Poller = &tb.LongPoller{
		Timeout: 10 * time.Second,
	}

	var webhook bool

	// You can use telegram webhooks instead long pooling if you generate self signed certs
	if _, err := os.Stat("cert.pem"); err == nil && host != "localhost" {
		log.Println("tg webhook mode")

		var whTLS *tb.WebhookTLS
		endp := &tb.WebhookEndpoint{
			PublicURL: fmt.Sprintf("https://%s", host),
		}

		whTLS = &tb.WebhookTLS{
			Key:  "key.pem",
			Cert: "cert.pem",
		}
		endp.Cert = "cert.pem"

		poller = &tb.Webhook{
			Listen:        ":8443",
			Endpoint:      endp,
			TLS:           whTLS,
			HasCustomCert: true,
		}

		webhook = true
		log.Printf("Telegram bot webhooks set on %s:8443 \n", endp.PublicURL)
	}

	b, err := tb.NewBot(tb.Settings{
		Token:  token,
		Poller: poller,
	})

	if !webhook {
		if err = b.RemoveWebhook(); err != nil {
			log.Fatalln(err)
		}
	}

	return &TgBot{
		token: token,
		host:  host,
		group: fmt.Sprintf("%d", group),
		owner: fmt.Sprintf("%d", owner),
		cb:    cb,
		tg:    b,
	}, nil
}

func (bot *TgBot) startTgBot() {
	var err error

	bot.tg.Handle("/getlink", func(m *tb.Message) {
		if !m.Private() {
			return
		}

		if bot.checkExist(m.Sender.ID) {
			err = bot.setRights(m.Sender.ID, false)
			bot.send(m.Sender, "you already linked to group")
			return
		}

		response, errC := bot.cb(CommandGetLink, cdata{UserID: m.Sender.ID})
		if errC != nil {
			bot.sendErr(m, err)
			return
		}

		bot.send(m.Sender, response, tb.ModeMarkdownV2, tb.NoPreview)
	})

	bot.tg.Handle("/add", func(m *tb.Message) {
		if !m.Private() {
			return
		}
		if m.Sender.Recipient() != bot.owner {
			return
		}

		bot.waitingID = true
		bot.send(m.Sender, "Send me user ID")
	})

	bot.tg.Handle(tb.OnText, func(m *tb.Message) {
		if !m.Private() {
			return
		}
		if m.Sender.Recipient() != bot.owner {
			return
		}
		if !bot.waitingID {
			return
		}
		bot.waitingID = false

		if !CheckNumericOnly(m.Text) {
			bot.send(m.Sender, "ID must be a numeric")
		}

		id, errC := strconv.Atoi(m.Text)
		if errC != nil {
			bot.send(m.Sender, err.Error())
			return
		}

		chat, err := bot.tg.ChatByID(bot.group)
		if err != nil {
			bot.send(m.Sender, err.Error())
			return
		}

		member, err := bot.tg.ChatMemberOf(chat, &tb.User{
			ID: id,
		})
		if err != nil {
			bot.send(m.Sender, err.Error())
			return
		}

		response, errC := bot.cb(CommandAddWhiteList, cdata{UserID: id})
		if errC != nil {
			bot.send(m.Sender, err.Error())
			return
		}

		bot.send(m.Sender, response)
		bot.send(member.User, "you are in white list!")
	})

	// Update new user permissions
	bot.tg.Handle(tb.OnUserJoined, func(m *tb.Message) {
		if m.Chat.Recipient() != bot.group {
			return
		}
		var ids []int
		if m.UserJoined != nil {
			if m.UserJoined.IsBot {
				return
			}
			ids = append(ids, m.UserJoined.ID)
		} else {
			for _, user := range m.UsersJoined {
				if user.IsBot {
					continue
				}
				ids = append(ids, user.ID)
			}
		}

		for _, id := range ids {
			if bot.checkExist(id) {
				continue
			}

			log.Printf("New user [%v], set restrict\n", id)

			err = bot.setRights(id, true)
			if err != nil {
				log.Println("ERROR:", err)
			}
		}
	})

	/*
		// Check if group is ours
		b.Handle(tb.OnAddedToGroup, func(m *tb.Message) {
			panic("not implemented")
		})

		//Message in group

		b.Handle(tb.OnText, func(m *tb.Message) {
			b.Send(m.Sender, "Hello World!")
		})

		b.Handle(tb.OnAudio, func(m *tb.Message) {
			b.Send(m.Sender, "Hello World!")
		})

		b.Handle(tb.OnAnimation, func(m *tb.Message) {
			b.Send(m.Sender, "Hello World!")
		})

		b.Handle(tb.OnVideo, func(m *tb.Message) {
			b.Send(m.Sender, "Hello World!")
		})

		b.Handle(tb.OnSticker, func(m *tb.Message) {
			b.Send(m.Sender, "Hello World!")
		})

		b.Handle(tb.OnLocation, func(m *tb.Message) {
			b.Send(m.Sender, "Hello World!")
		})

		b.Handle(tb.OnContact, func(m *tb.Message) {
			b.Send(m.Sender, "Hello World!")
		})

		b.Handle(tb.OnVideoNote, func(m *tb.Message) {
			b.Send(m.Sender, "Hello World!")
		})

		b.Handle(tb.OnDocument, func(m *tb.Message) {
			b.Send(m.Sender, "Hello World!")
		})

		b.Handle(tb.OnPhoto, func(m *tb.Message) {
			b.Send(m.Sender, "Hello World!")
		})
	*/

	bot.tg.Start()
}

func (bot *TgBot) checkExist(id int) bool {
	r, err := bot.cb(CommandCheckWhiteList, cdata{UserID: id})
	if err != nil {
		log.Println("ERROR: ", err)
	}
	if r == "exist" {
		return true
	}
	r, err = bot.cb(CommandCheckUser, cdata{UserID: id})
	if err != nil {
		log.Println("ERROR: ", err)
	}
	if r == "exist" {
		return true
	}

	return false
}

func (bot *TgBot) setRights(userID int, mute bool) error {

	chat, err := bot.tg.ChatByID(bot.group)
	if err != nil {
		return err
	}

	member, err := bot.tg.ChatMemberOf(chat, &tb.User{
		ID: userID,
	})
	if err != nil {
		return err
	}

	if mute {
		member.Rights.CanSendOther = false
		member.Rights.CanSendMedia = false
		member.Rights.CanSendPolls = false
		member.Rights.CanSendMessages = false
	} else {
		member.Rights.CanSendOther = true
		member.Rights.CanSendMedia = true
		member.Rights.CanSendPolls = true
		member.Rights.CanSendMessages = true
	}
	member.RestrictedUntil = time.Now().Unix()

	err = bot.tg.Restrict(chat, member)
	if err != nil {
		return err
	}

	if !mute {
		bot.send(member.User, "Now you can send message in group")
	} else {
		bot.send(member.User, "You rights has been restricted in group")
	}

	return nil
}

func (bot *TgBot) send(r tb.Recipient, msg string, options ...interface{}) {
	_, err := bot.tg.Send(r, msg, options...)
	if err != nil {
		log.Println("ERROR [SEND]: ", err)
	}
}

func (bot *TgBot) sendErr(m *tb.Message, err error) {
	switch err {
	default:
		bot.send(m.Sender, "Error happen! 😟")
	}
}
