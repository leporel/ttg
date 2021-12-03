package main

import (
	"flag"
	"fmt"
	"github.com/patrickmn/go-cache"
	"log"
	"sync"
	"time"
)

type RestrictMode int

const (
	RestrictFollowers   RestrictMode = iota
	RestrictSubscribers              // TODO
)

type config struct {
	TwitchAppID       string
	TwitchSecCode     string
	TwitchChannelName string

	TelegramBotToken string
	TelegramGroup    int
	TelegramOwner    int

	// TODO  First init
	Init bool

	Restrict RestrictMode

	Host string
}

func main() {
	cfg, err := loadConfig()
	if err != nil {
		log.Fatalln(err)
	}

	bot := &TTG{
		cfg: cfg,
		mu:  &sync.Mutex{},
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

	bot.cache = cache.New(time.Minute*10, time.Minute*1)

	bot.start()
}

func loadConfig() (*config, error) {
	var cfg config

	flag.StringVar(&cfg.Host, "host", "", "Host where you run this bot (IP or URL)")

	flag.StringVar(&cfg.TwitchAppID, "app", "", "Twitch app id")
	flag.StringVar(&cfg.TwitchSecCode, "code", "", "Twitch app secret code")
	flag.StringVar(&cfg.TwitchChannelName, "channel", "", "Your channel name")

	flag.IntVar(&cfg.TelegramGroup, "group", 0, "Your telegram group(chat) id")
	flag.IntVar(&cfg.TelegramOwner, "owner", 0, "Your telegram user id")
	flag.StringVar(&cfg.TelegramBotToken, "token", "", "Telegram bot token")

	flag.Parse()

	if cfg.Host == "" {
		return nil, fmt.Errorf("missing host")
	}
	if cfg.TwitchAppID == "" {
		return nil, fmt.Errorf("missing app")
	}
	if cfg.TwitchSecCode == "" {
		return nil, fmt.Errorf("missing code")
	}
	if cfg.TwitchChannelName == "" {
		return nil, fmt.Errorf("missing channel")
	}
	if cfg.TelegramGroup == 0 {
		return nil, fmt.Errorf("missing group")
	}
	if cfg.TelegramBotToken == "" {
		return nil, fmt.Errorf("missing token")
	}

	//if !CheckNumericOnly(cfg.TwitchChannelID) {
	//	return nil, fmt.Errorf("TwitchChannelID key must be only numeric: '%s'", cfg.TwitchChannelID)
	//}

	return &cfg, nil
}
