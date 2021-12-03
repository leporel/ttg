package main

import (
	"fmt"
	"github.com/nicklaw5/helix/v2"
	"log"
	"sync"
	"time"
)

type TwitchApp struct {
	clientUser *helix.Client
	clientApp  *helix.Client

	mu *sync.RWMutex

	appToken     helix.AccessCredentials
	appTokenDate time.Time

	clientID string
	apiCode  string
	host     string

	broadcasterID string
}

func NewTwitchClient(channelName, cliID, code, host string) (*TwitchApp, error) {
	schema := "https"

	if host == "localhost" {
		schema = "http"
	}

	clientApp, err := helix.NewClient(&helix.Options{
		ClientID:      cliID,
		ClientSecret:  code,
		RedirectURI:   fmt.Sprintf("%s://%s:8444/auth/callback", schema, host),
		RateLimitFunc: rateLimitCallback,
	})
	if err != nil {
		return nil, err
	}

	clientUser, err := helix.NewClient(&helix.Options{
		ClientID:      cliID,
		ClientSecret:  code,
		RedirectURI:   fmt.Sprintf("%s://%s:8444/auth/callback", schema, host),
		RateLimitFunc: rateLimitCallback,
	})
	if err != nil {
		return nil, err
	}

	app := &TwitchApp{
		clientApp:    clientApp,
		clientUser:   clientUser,
		clientID:     cliID,
		apiCode:      code,
		appTokenDate: time.Now(),
		host:         host,
		mu:           &sync.RWMutex{},
	}

	token, err := app.refreshAppToken()
	if err != nil {
		return nil, err
	}

	channelID := ""

	clientApp.SetAppAccessToken(token)
	users, err := clientApp.GetUsers(&helix.UsersParams{
		Logins: []string{channelName},
	})
	if err != nil {
		return nil, err
	}

	if len(users.Data.Users) == 0 {
		return nil, fmt.Errorf("not found twitch channel %s", channelName)
	}

	channelID = users.Data.Users[0].ID

	log.Println("Twitch channel id: ", channelName, channelID)

	app.broadcasterID = channelID

	return app, nil
}

func rateLimitCallback(lastResponse *helix.Response) error {
	if lastResponse.GetRateLimitRemaining() > 0 {
		return nil
	}

	var reset64 int64
	reset64 = int64(lastResponse.GetRateLimitReset())

	currentTime := time.Now().Unix()

	if currentTime < reset64 {
		timeDiff := time.Duration(reset64 - currentTime)
		if timeDiff > 0 {
			log.Printf("Waiting on rate limit to pass before sending next request (%d seconds)\n", timeDiff)
			time.Sleep(timeDiff * time.Second)
		}
	}

	return nil
}

func (t *TwitchApp) refreshAppToken() (string, error) {
	if t.appToken.AccessToken == "" || t.appTokenDate.Add(time.Second*time.Duration(t.appToken.ExpiresIn)).After(time.Now()) {

		resp, err := t.clientApp.RequestAppAccessToken([]string{"user:read:follows"})
		if err != nil {
			return "", err
		}
		t.appTokenDate = time.Now()
		t.appToken = resp.Data
	}

	return t.appToken.AccessToken, nil
}

func (t *TwitchApp) getAuthLink(uniqueID string) (string, error) {
	url := t.clientUser.GetAuthorizationURL(&helix.AuthorizationURLParams{
		ResponseType: "code",
		Scopes:       []string{"user:read:follows", "user:read:subscriptions"},
		State:        uniqueID,
		ForceVerify:  false,
	})

	return url, nil
}

func (t *TwitchApp) getUserToken(code string) (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	resp, err := t.clientUser.RequestUserAccessToken(code)
	if err != nil {
		return "", err
	}

	return resp.Data.AccessToken, nil
}

// User self information by his token
func (t *TwitchApp) getUser(token string) (*helix.User, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.clientUser.SetUserAccessToken(token)

	resp, err := t.clientUser.GetUsers(&helix.UsersParams{
		IDs:    []string{},
		Logins: []string{},
	})
	if err != nil {
		return nil, err
	}

	return &resp.Data.Users[0], nil
}

// Get user follows
// return map[channelID]channelName
func (t *TwitchApp) getFollows(twitchID string) (map[string]string, error) {
	_, err := t.refreshAppToken()
	if err != nil {
		return nil, err
	}

	t.clientApp.SetAppAccessToken(t.appToken.AccessToken)

	rs := make(map[string]string, 0)

	cursor := ""

infinity:
	for true {
		resp, err := t.clientApp.GetUsersFollows(&helix.UsersFollowsParams{
			After:  cursor,
			First:  100,
			FromID: twitchID,
		})
		if err != nil {
			return nil, err
		}

		if resp.Error != "" {
			return nil, fmt.Errorf("%s", resp.ErrorMessage)
		}

		for _, flw := range resp.Data.Follows {
			rs[flw.ToID] = flw.ToName
		}

		if resp.Data.Pagination.Cursor == "" {
			break infinity
		}
		cursor = resp.Data.Pagination.Cursor
	}

	return rs, nil
}

// Get channel followers
// return map[twitchID]twitchName
func (t *TwitchApp) getFollowers() (Followers, error) {
	_, err := t.refreshAppToken()
	if err != nil {
		return nil, err
	}

	t.clientApp.SetAppAccessToken(t.appToken.AccessToken)

	rs := make(map[string]string, 0)

	cursor := ""

infinity:
	for true {
		resp, err := t.clientApp.GetUsersFollows(&helix.UsersFollowsParams{
			After: cursor,
			First: 100,
			ToID:  t.broadcasterID,
		})
		if err != nil {
			return nil, err
		}

		if resp.Error != "" {
			return nil, fmt.Errorf("%s", resp.ErrorMessage)
		}

		for _, flw := range resp.Data.Follows {
			rs[flw.FromID] = flw.FromLogin
		}

		if resp.Data.Pagination.Cursor == "" {
			break infinity
		}
		cursor = resp.Data.Pagination.Cursor
	}

	return rs, nil
}
