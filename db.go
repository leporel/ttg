package main

import (
	"database/sql"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"log"
	"strings"
	"time"
)

type Storage struct {
	db *sql.DB
}

type User struct {
	TelegramID int
	TwitchID   int
	Name       string
	CreatedAt  time.Time
}

type WhiteListedUser struct {
	TelegramID  int
	Description string
}

func NewStorage() (*Storage, error) {
	db, err := sql.Open("sqlite3", "db.sqlite")
	if err != nil {
		log.Fatal(err)
	}

	tables, err := db.Query("select name FROM sqlite_schema WHERE type = ? AND name NOT LIKE ?", "table", "sqlite_%")
	if err != nil {
		return nil, err
	}

	var exist []string
	for tables.Next() {
		var name string
		err = tables.Scan(&name)
		if err != nil {
			return nil, err
		}
		exist = append(exist, name)
	}
	err = tables.Err()
	if err != nil {
		return nil, err
	}
	err = tables.Close()
	if err != nil {
		log.Fatal(err)
	}

	if !strings.Contains(strings.Join(exist, ","), "whitelist") {

		sqlStmt := `
		drop table if exists whitelist;
		create table whitelist (tg_id integer not null primary key, description text);
		delete from whitelist;
		`
		_, err = db.Exec(sqlStmt)
		if err != nil {
			return nil, err
		}

		log.Println("New database created")
	}

	if !strings.Contains(strings.Join(exist, ","), "followers") {

		sqlStmt := `
		drop table if exists followers;
		create table followers (tg_id integer not null primary key, twitch_id integer not null, name text not null, created_at timestamp not null);
        CREATE INDEX idx_twitch_id  ON followers(twitch_id);
		delete from followers;
		`
		_, err = db.Exec(sqlStmt)
		if err != nil {
			return nil, err
		}

		log.Println("New database created")
	}

	return &Storage{
		db: db,
	}, nil
}

func (s *Storage) AddWhiteList(user *WhiteListedUser) error {
	_, err := s.db.Exec("INSERT OR IGNORE into whitelist(tg_id, description) values(?, ?)",
		user.TelegramID, user.Description)
	if err != nil {
		return err
	}
	row := s.db.QueryRow("select tg_id from whitelist where tg_id = ?", user.TelegramID)
	if row.Err() != nil {
		return err
	}
	var id int
	err = row.Scan(&id)
	if err != nil {
		return err
	}
	if id != user.TelegramID {
		return fmt.Errorf("row not inserted")
	}

	return nil
}

func (s *Storage) GetWhiteListedUser(tgID int) (*WhiteListedUser, error) {

	row := s.db.QueryRow("select tg_id, description from whitelist where tg_id = ?", tgID)
	if row.Err() != nil {
		return nil, row.Err()
	}
	u := &WhiteListedUser{}
	err := row.Scan(&u.TelegramID, &u.Description)
	if err != nil {
		return nil, err
	}
	if tgID != u.TelegramID {
		return nil, sql.ErrNoRows
	}

	return u, nil
}

func (s *Storage) DeleteWhiteListedUser(tgID int) error {

	affect, err := s.db.Exec("delete from whitelist where tg_id=?", tgID)
	if err != nil {
		return err
	}

	afc, err := affect.RowsAffected()
	if err != nil {
		return err
	}
	if afc == 0 {
		log.Fatal(fmt.Errorf("delete not affected"))
	}

	return nil
}

func (s *Storage) AddUser(user *User) error {
	_, err := s.db.Exec("INSERT OR IGNORE into followers(tg_id, twitch_id, name, created_at) values(?, ?, ?, ?)",
		user.TelegramID, user.TwitchID, user.Name, user.CreatedAt)
	if err != nil {
		return err
	}
	row := s.db.QueryRow("select tg_id from followers where tg_id = ?", user.TelegramID)
	if row.Err() != nil {
		return err
	}
	var id int
	err = row.Scan(&id)
	if err != nil {
		return err
	}
	if id != user.TelegramID {
		return fmt.Errorf("row not inserted")
	}

	return nil
}

func (s *Storage) GetUserByTgId(tgID int) (*User, error) {

	row := s.db.QueryRow("select tg_id, twitch_id, name, created_at from followers where tg_id = ?", tgID)
	if row.Err() != nil {
		return nil, row.Err()
	}
	u := &User{}
	err := row.Scan(&u.TelegramID, &u.TwitchID, &u.Name, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	if tgID != u.TelegramID {
		return nil, sql.ErrNoRows
	}

	return u, nil
}

func (s *Storage) GetUserByTwId(twID int) (*User, error) {

	row := s.db.QueryRow("select tg_id, twitch_id, name, created_at from followers where twitch_id = ?", twID)
	if row.Err() != nil {
		return nil, row.Err()
	}
	u := &User{}
	err := row.Scan(&u.TelegramID, &u.TwitchID, &u.Name, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	if twID != u.TwitchID {
		return nil, sql.ErrNoRows
	}

	return u, nil
}

func (s *Storage) DeleteUser(tgID int) error {

	affect, err := s.db.Exec("delete from followers where tg_id=?", tgID)
	if err != nil {
		return err
	}

	afc, err := affect.RowsAffected()
	if err != nil {
		return err
	}
	if afc == 0 {
		log.Fatal(fmt.Errorf("delete not affected"))
	}

	return nil
}

// GetUsers return map[twitchID]telegramID
func (s *Storage) GetUsers() (map[string]int, error) {

	rows, err := s.db.Query("SELECT tg_id, twitch_id FROM followers")
	if err != nil {
		return nil, err
	}
	users := make(map[string]int, 0)

	var tw string
	var tg int

	for rows.Next() {
		err = rows.Scan(&tg, &tw)
		if err != nil {
			return nil, err
		}
		users[tw] = tg
	}

	err = rows.Err()
	if err != nil {
		return nil, err
	}

	err = rows.Close()
	if err != nil {
		log.Fatal(err)
	}

	return users, nil
}
