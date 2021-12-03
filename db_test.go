package main

import (
	"testing"
	"time"
)

func TestStorage(t *testing.T) {
	_, err := NewStorage()

	if err != nil {
		t.Fatal(err)
	}
}

func TestStorageWhiteList(t *testing.T) {
	db, err := NewStorage()

	if err != nil {
		t.Fatal(err)
	}

	err = db.AddWhiteList(&WhiteListedUser{2231231, "old"})

	if err != nil {
		t.Fatal(err)
	}

	user, err := db.GetWhiteListedUser(2231231)
	if err != nil {
		t.Fatal(err)
	}

	t.Log(user)

	err = db.DeleteWhiteListedUser(2231231)
	if err != nil {
		t.Fatal(err)
	}
}

func TestStorageUser(t *testing.T) {
	db, err := NewStorage()

	if err != nil {
		t.Fatal(err)
	}

	err = db.AddUser(&User{
		TelegramID: 34235324,
		TwitchID:   12312,
		Name:       "Arthas",
		CreatedAt:  time.Now(),
	})

	if err != nil {
		t.Fatal(err)
	}

	user, err := db.GetUserByTgId(34235324)
	if err != nil {
		t.Fatal(err)
	}

	t.Log(user)

	err = db.DeleteUser(34235324)
	if err != nil {
		t.Fatal(err)
	}
}

func TestStorageGetUsers(t *testing.T) {
	db, err := NewStorage()

	if err != nil {
		t.Fatal(err)
	}

	users, err := db.GetUsers()
	if err != nil {
		t.Fatal(err)
	}

	t.Log(users)
}
