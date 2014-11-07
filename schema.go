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
