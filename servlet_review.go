package main

import (
	"fmt"
	"net/http"
	"time"
)

type ReviewServlet struct {
	db              *sql.DB
	session_manager *SessionManager
}

type Review struct {
	Id           int
	User_id      int
	Date         time.Time
	Review       string
	Title        string
	Professor_id int
	Class_id     int
	Recommend    bool
}

func NewReviewServlet() *ReviewServlet {
	t := new(ReviewServlet)
	t.session_manager = session_manager

	db, err := sql.Open("mysql", server_config.GetSqlURI())
	if err != nil {
		log.Fatal("NewUserServlet", "Failed to open database:", err)
	}
	t.db = db
	return t
}

func (t *ReviewServlet) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	HandleServletRequest(t, w, r)
}
