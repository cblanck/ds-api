package main

import (
	"time"
)

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

type Session struct {
	User    *UserData
	Expires time.Time
}
