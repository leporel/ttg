package main

import (
	"database/sql"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/patrickmn/go-cache"
	"github.com/throttled/throttled/v2"
	"github.com/throttled/throttled/v2/store/memstore"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"
)

type Handler func(http.ResponseWriter, *http.Request) error

type Followers map[string]string

type TTG struct {
	cfg   *config
	app   *TwitchApp
	tg    *TgBot
	db    *Storage
	ready bool
	mu    *sync.Mutex

	cache *cache.Cache
}

func (b *TTG) start() {
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	ticker := time.NewTicker(30 * time.Minute)

	go func() {
		b.tg.startTgBot()
	}()

	go func() {
		for {
			select {
			case _ = <-ticker.C:
				if err := b.checkPermissions(); err != nil {
					log.Println("ERROR: ", err)
				}
			}
		}
	}()

	b.ready = true

	go func() {
		b.listen()
	}()

	<-c
	ticker.Stop()
}

func (b *TTG) listen() {
	store, err := memstore.New(65536)
	if err != nil {
		log.Fatal(err)
	}
	quota := throttled.RateQuota{
		MaxRate:  throttled.PerMin(30),
		MaxBurst: 5,
	}
	rateLimiter, err := throttled.NewGCRARateLimiter(store, quota)
	if err != nil {
		log.Fatal(err)
	}
	httpRateLimiter := throttled.HTTPRateLimiter{
		DeniedHandler: throttled.DefaultDeniedHandler,
		RateLimiter:   rateLimiter,
		VaryBy:        &throttled.VaryBy{RemoteAddr: true},
	}

	// https://github.com/twitchdev/authentication-go-sample/blob/main/oauth-authorization-code/main.go
	var middleware = func(h Handler) Handler {
		return func(w http.ResponseWriter, r *http.Request) (err error) {
			// parse POST body
			if err = r.ParseForm(); err != nil {
				return err
			}

			return h(w, r)
		}
	}

	var errorHandling = func(handler func(w http.ResponseWriter, r *http.Request) error) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := handler(w, r); err != nil {
				var errorString string = "Something went wrong..."
				var errorCode int = 500

				log.Println(err)
				w.WriteHeader(errorCode)
				if _, err = w.Write([]byte(errorString)); err != nil {
					log.Println("ERROR: ", err)
				}
				return
			}
		})
	}

	var handleFunc = func(path string, handler Handler) {
		http.Handle(path, httpRateLimiter.RateLimit(errorHandling(middleware(handler))))
	}

	log.Printf("Started listening on http://localhost:8444 \n")
	handleFunc("/auth/callback", b.handleOAuth2Callback)

	log.Println(http.ListenAndServe(":8444", nil))
}

func (b *TTG) handleOAuth2Callback(w http.ResponseWriter, r *http.Request) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if !b.ready {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`<html><body>Bot not ready</body></html>`)); err != nil {
			log.Println("ERROR: ", err)
		}
	}

	switch state := r.FormValue("state"); {
	case state == "":
		return errors.New("missing state challenge")
	case state != "":
		if _, err := uuid.Parse(state); err != nil {
			return fmt.Errorf("wrong state")
		}

		tgIDi, found := b.cache.Get(state)
		if !found {
			return fmt.Errorf("missing state in cache")
		}

		tgID := tgIDi.(int)

		found, err := b.checkUserTelegram(tgID)
		if err != nil {
			return err
		}
		if found {
			return fmt.Errorf("user exist (tg id)")
		}

		accessToken, err := b.app.getUserToken(r.FormValue("code"))
		if err != nil {
			return err
		}

		user, err := b.app.getUser(accessToken)
		if err != nil {
			return err
		}
		twID, err := strconv.Atoi(user.ID)
		if err != nil {
			return err
		}

		found, err = b.checkUserTwitch(twID)
		if err != nil {
			return err
		}
		if found {
			return fmt.Errorf("user exist (tw id)")
		}

		channels, err := b.app.getFollows(user.ID)

		_, found = channels[b.app.broadcasterID]
		if !found {
			w.WriteHeader(http.StatusForbidden)
			if _, err := w.Write([]byte(`<html><body>Authorization successful, but channel not found in your followed list</body></html>`)); err != nil {
				log.Println("ERROR: ", err)
			}

			return nil
		}

		err = b.addUser(tgID, twID, user.DisplayName)
		if err != nil {
			return err
		}
	}

	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte(`<html><body>Authorization successful, bot will soon give to you rights</body></html>`)); err != nil {
		log.Println("ERROR: ", err)
	}

	return nil
}

func (b *TTG) commandHandler(command COMMAND, payload cdata) (string, error) {
	if !b.ready {
		return "Bot not ready", nil
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	switch command {
	case CommandAddWhiteList:
		err := b.addWhiteList(payload.UserID, "manual added")
		if err != nil {
			return "", err
		}

		return fmt.Sprintf("User %v added in white list", payload.UserID), nil

	case CommandGetLink:

		uid := uuid.New().String()

		err := b.cache.Add(fmt.Sprint(payload.UserID), uid, cache.DefaultExpiration)
		if err == nil {
			err := b.cache.Add(uid, payload.UserID, cache.DefaultExpiration)
			if err != nil {
				return "", err
			}
		} else {
			uidI, found := b.cache.Get(fmt.Sprint(payload.UserID))
			if !found {
				return "", fmt.Errorf("uid cache not found")
			}
			uid = uidI.(string)
		}

		link, err := b.app.getAuthLink(uid)
		if err != nil {
			return "", err
		}

		rsp := fmt.Sprintf("Link: [click me](%s) \n\n Link live 10 minutes, after time expired, your need to get one new", link)

		return rsp, nil

	case CommandCheckWhiteList:
		exist, err := b.checkWhiteList(payload.UserID)
		if err != nil {
			return "", err
		}
		if exist {
			return "exist", nil
		}
		return "none", nil

	case CommandCheckUser:
		exist, err := b.checkUserTelegram(payload.UserID)
		if err != nil {
			return "", err
		}
		if exist {
			return "exist", nil
		}
		return "none", nil
	}

	return "Unknown command", nil
}

func (b *TTG) checkPermissions() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	users, err := b.db.GetUsers()
	if err != nil {
		if err == sql.ErrNoRows {
			return nil
		}
		return err
	}

	if len(users) == 0 {
		return nil
	}

	followers, err := b.app.getFollowers()
	if err != nil {
		return err
	}

	for twitchID, tgID := range users {
		if _, exist := followers[twitchID]; !exist {
			log.Printf("Not found as follower: %s \n", twitchID)
			if err := b.removeUser(tgID); err != nil {
				log.Println("ERROR: ", err)
			}
		}
	}

	return nil
}

func (b *TTG) addUser(tgID, twID int, name string) error {
	log.Printf("Add user [%s]\n", name)

	err := b.db.AddUser(&User{
		TelegramID: tgID,
		TwitchID:   twID,
		Name:       name,
		CreatedAt:  time.Now(),
	})
	if err != nil {
		return err
	}

	err = b.tg.setRights(tgID, false)
	if err != nil {
		return err
	}

	return nil
}

func (b *TTG) removeUser(tgID int) error {
	log.Printf("Remove user id [%v]\n", tgID)

	err := b.db.DeleteUser(tgID)
	if err != nil {
		return err
	}

	err = b.tg.setRights(tgID, true)
	if err != nil {
		return err
	}

	return nil
}

func (b *TTG) addWhiteList(userID int, dcs string) error {
	err := b.db.AddWhiteList(&WhiteListedUser{
		TelegramID:  userID,
		Description: dcs,
	})
	if err != nil {
		return err
	}

	err = b.tg.setRights(userID, false)
	if err != nil {
		return err
	}

	return nil
}

func (b *TTG) checkWhiteList(userID int) (bool, error) {
	rs, err := b.db.GetWhiteListedUser(userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	if rs != nil {
		return true, nil
	}

	return false, nil
}

func (b *TTG) checkUserTelegram(userID int) (bool, error) {
	rs, err := b.db.GetUserByTgId(userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	if rs != nil {
		return true, nil
	}

	return false, nil
}

func (b *TTG) checkUserTwitch(userID int) (bool, error) {
	rs, err := b.db.GetUserByTwId(userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	if rs != nil {
		return true, nil
	}

	return false, nil
}
