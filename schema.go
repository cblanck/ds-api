package main

import (
	"database/sql"
	"time"
)

/*
 * Users
 */
type UserData struct {
	Id                 int
	Username           string
	password           string
	password_salt      string
	Email              string
	First_name         string
	Last_name          string
	Class_year         string
	Account_created    time.Time
	Last_login         time.Time
	Session_token      string
	password_reset_key string
}

// Fetches information about a user by username.
func GetUserByName(db *sql.DB, username string) (*UserData, error) {
	row := db.QueryRow(`SELECT id, username, password, password_salt,
		email, first_name, last_name, class_year, account_created, last_login,
		password_reset_key FROM degreesheep.user WHERE username = ?`, username)

	user_data := new(UserData)
	if err := row.Scan(
		&user_data.Id,
		&user_data.Username,
		&user_data.password,
		&user_data.password_salt,
		&user_data.Email,
		&user_data.First_name,
		&user_data.Last_name,
		&user_data.Class_year,
		&user_data.Account_created,
		&user_data.Last_login,
		&user_data.password_reset_key); err != nil {
		return nil, err
	}

	return user_data, nil
}

// Get information for a user by UID
func GetUserById(db *sql.DB, uid int) (*UserData, error) {
	row := db.QueryRow(`SELECT id, username, password, password_salt,
		email, first_name, last_name, class_year, account_created, last_login,
		password_reset_key FROM degreesheep.user WHERE id = ?`, uid)

	user_data := new(UserData)
	if err := row.Scan(
		&user_data.Id,
		&user_data.Username,
		&user_data.password,
		&user_data.password_salt,
		&user_data.Email,
		&user_data.First_name,
		&user_data.Last_name,
		&user_data.Class_year,
		&user_data.Account_created,
		&user_data.Last_login,
		&user_data.password_reset_key); err != nil {
		return nil, err
	}
	return user_data, nil
}

type Session struct {
	User    *UserData
	Expires time.Time
}

type Class struct {
	Id               int
	Subject          int
	Subject_callsign string
	Course_number    int
	Description      string
}

type Instructor struct {
	Id    int
	Name  string
	Email string
}

type Comment struct {
	Id        int
	Review_id int
	User_id   int
	Date      time.Time
	Text      string
	User      *UserData
}

type Review struct {
	Id            int
	User_id       int
	Date          time.Time
	Review        string
	Title         string
	Instructor_id int
	Class_id      int
	Recommend     bool
	User          *UserData
	Instructor    *Instructor
	Comments      []*Comment
}
