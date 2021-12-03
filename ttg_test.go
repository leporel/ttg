package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"
)

func newTTG() *TTG {

	owner, _ := strconv.Atoi(os.Getenv("TelegramOwner"))

	cfg := &config{
		TwitchAppID:       os.Getenv("TwitchAppID"),
		TwitchSecCode:     os.Getenv("TwitchSecCode"),
		TwitchChannelName: "leporel",
		TelegramBotToken:  os.Getenv("TelegramBotToken"),
		TelegramGroup:     -1001575283959,
		TelegramOwner:     owner,
		Init:              false,
		Host:              "localhost",
	}

	bot := &TTG{
		cfg: cfg,
	}

	db, err := NewStorage()
	if err != nil {
		log.Fatalln(err)
	}
	bot.db = db

	app, err := NewTwitchClient(cfg.TwitchChannelName, cfg.TwitchAppID, cfg.TwitchSecCode, cfg.Host)
	if err != nil {
		log.Fatalln(err)
	}
	bot.app = app

	tg, err := NewTgBot(cfg.TelegramBotToken, cfg.TelegramGroup, cfg.TelegramOwner, cfg.Host, bot.commandHandler)
	if err != nil {
		log.Fatalln(err)
	}
	bot.tg = tg

	return bot
}

func TestTTG_fetchFollowers(t *testing.T) {
	ttg := newTTG()

	fwls, err := ttg.app.getFollowers()
	if err != nil {
		t.Fatal(err)
	}

	var i = 0
	for id, name := range fwls {
		if i > 100 {
			break
		}

		t.Log(id, name)

		i++
	}

}

func TestTTG_fetchTwitchUserInfo(t *testing.T) {
	ttg := newTTG()
	srv := &http.Server{Addr: ":8444"}

	link, err := ttg.app.getAuthLink("test")
	if err != nil {
		t.Fatal(err)
	}
	log.Println(link)

	hnd := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")

		state := r.FormValue("state")

		if state != "test" {
			t.Fatal("wrong state")
		}

		accessToken, err := ttg.app.getUserToken(r.FormValue("code"))
		if err != nil {
			t.Fatal(err)
		}

		user, err := ttg.app.getUser(accessToken)
		if err != nil {
			t.Fatal(err)
		}
		t.Log(user)

		channels, err := ttg.app.getFollows(user.ID)
		for s, s2 := range channels {
			t.Log(s, s2)
		}

		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`<html><body>Authorization successful, bot will soon give to you rights</body></html>`)); err != nil {
			log.Println("ERROR: ", err)
		}

		go func() {
			time.Sleep(1 * time.Second)

			srv.Shutdown(context.Background())
		}()

		return
	})

	log.Printf("Started running on http://localhost:8444 \n")
	http.Handle("/auth/callback", hnd)

	log.Println(srv.ListenAndServe())
}
