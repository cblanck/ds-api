package main

import (
	"database/sql"
	_ "github.com/go-sql-driver/mysql"
	"log"
	"net/http"
	"time"
)

type ReviewServlet struct {
	db              *sql.DB
	session_manager *SessionManager
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
}

func NewReviewServlet(server_config Config, session_manager *SessionManager) *ReviewServlet {
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

func (t *ReviewServlet) List(w http.ResponseWriter, r *http.Request) {
	class_id := r.Form.Get("class_id")

	if class_id == "" {
		ServeError(w, r, "Missing value for one or more fields", 200)
		return
	}

	rows, err := t.db.Query("SELECT id, title, recommend FROM review WHERE class_id = ?", class_id)
	if err != nil {
		log.Println("List", err)
		ServeError(w, r, "Internal server error", 500)
		return
	}

	defer rows.Close()
	review_list := make([]*Review, 0)
	for rows.Next() {
		review := new(Review)
		if err := rows.Scan(
			&review.Id,
			&review.Title,
			&review.Recommend); err != nil {
			log.Println("List", err)
			ServeError(w, r, "Internal server error", 500)
		}
		review_list = append(review_list, review)
	}
	ServeResult(w, r, review_list)
}

func (t *ReviewServlet) Review(w http.ResponseWriter, r *http.Request) {
	id := r.Form.Get("Id")

	if id == "" {
		ServeError(w, r, "Missing value for one or more fields", 200)
		return
	}

	rows, err := t.db.Query("SELECT id, user_id, date, review, title, professor_id, class_id, recommend FROM review WHERE id = ?", id)

	if err != nil {
		log.Println("Review", err)
		ServeError(w, r, "Internal server error", 500)
		return
	}

	defer rows.Close()
	review := make(Review)
	for rows.Next() {
		if err := rows.Scan(
			&review.Id,
			&review.User_id,
			&review.Date,
			&review.Title,
			&review.Professor_id,
			&review.Class_id,
			&review.Recommend); err != nil {
			log.Println("Review", err)
			ServeError(w, r, "Internal server error", 500)
		}
	}

	user, err := FetchUserById(t.db, review.User_id)
	if err != nil {
		log.Println("Review", err)
		ServeError(w, r, "Internal server error", 500)
		return
	}
	ServeResult(w, r, review)
}
