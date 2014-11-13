package main

import (
	"database/sql"
	_ "github.com/go-sql-driver/mysql"
	"log"
	"net/http"
)

type ReviewServlet struct {
	db              *sql.DB
	session_manager *SessionManager
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

func (t *ReviewServlet) ListReviews(w http.ResponseWriter, r *http.Request) {
	class_id := r.Form.Get("class_id")

	if class_id == "" {
		ServeError(w, r, "Missing value for one or more fields", 400)
		return
	}

	rows, err := t.db.Query("SELECT id, title, recommend FROM review WHERE class_id = ?", class_id)
	if err != nil {
		log.Println("ListReviews", err)
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
			log.Println("ListReviews", err)
			ServeError(w, r, "Internal server error", 500)
			return
		}
		review_list = append(review_list, review)
	}
	ServeResult(w, r, review_list)
}

func (t *ReviewServlet) PostReview(w http.ResponseWriter, r *http.Request) {
	session_id := r.Form.Get("session")

	session_valid, session, err := t.session_manager.GetSession(session_id)
	if err != nil {
		log.Println("PostReview", err)
		ServeError(w, r, "Internal server error", 500)
		return
	}
	if !session_valid {
		log.Println("PostReview", err)
		ServeError(w, r, "The specified session has expired", 401)
		return
	}

	review := r.Form.Get("review")
	title := r.Form.Get("title")
	instructor_id := r.Form.Get("instructor_id")
	class_id := r.Form.Get("class_id")
	recommend := r.Form.Get("recommend")

	if review == "" || title == "" || instructor_id == "" || class_id == "" ||
		recommend == "" {
		ServeError(w, r, "Missing value for one or more fields", 400)
		return
	}

	_, err = t.db.Exec(`INSERT INTO review (user_id, review, title,
                         instructor_id, class_id, recommend) VALUES (?, ?, ?, ?,?, ?)`,
		session.User.Id, review, title, instructor_id, class_id, recommend)
	if err != nil {
		log.Println("PostReview", err)
		ServeError(w, r, "Internal server error", 500)
		return
	}

	ServeResult(w, r, "OK")
}

func (t *ReviewServlet) PostComment(w http.ResponseWriter, r *http.Request) {
	session_id := r.Form.Get("session")

	session_valid, session, err := t.session_manager.GetSession(session_id)
	if err != nil {
		log.Println("PostComment", err)
		ServeError(w, r, "Internal server error", 500)
		return
	}
	if !session_valid {
		log.Println("PostComment", err)
		ServeError(w, r, "The specified session has expired", 401)
		return
	}

	review_id := r.Form.Get("review_id")
	text := r.Form.Get("text")

	if review_id == "" || text == "" {
		ServeError(w, r, "Missing value for one or more fields", 400)
		return
	}

	_, err = t.db.Exec(`INSERT INTO comment (review_id, user_id, text) VALUES
                        (?, ?, ?)`, review_id, session.User.Id, text)
	if err != nil {
		log.Println("PostComment", err)
		ServeError(w, r, "Internal server error", 500)
		return
	}

	ServeResult(w, r, "OK")
}

func FetchInstructorById(db *sql.DB, id int) (*Instructor, error) {
	instructor := new(Instructor)
	err := db.QueryRow("SELECT id, name, email FROM instructor WHERE id = ?", id).Scan(
		&instructor.Id,
		&instructor.Name,
		&instructor.Email,
	)
	if err != nil {
		return nil, err
	}
	return instructor, nil
}

func FetchCommentsByReviewId(db *sql.DB, id int) ([]*Comment, error) {
	rows, err := db.Query(`SELECT id, review_id, user_id, date, text FROM
                           comment WHERE review_id = ?`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	comments := make([]*Comment, 0)
	for rows.Next() {
		comment := new(Comment)
		if err := rows.Scan(
			&comment.Id,
			&comment.Review_id,
			&comment.User_id,
			&comment.Date,
			&comment.Text); err != nil {
			return nil, err
		}
		comment.User, err = GetUserById(db, comment.User_id)
		if err != nil {
			return nil, err
		}
		comments = append(comments, comment)
	}
	return comments, nil
}

func (t *ReviewServlet) GetReview(w http.ResponseWriter, r *http.Request) {
	id := r.Form.Get("review_id")

	if id == "" {
		ServeError(w, r, "Missing value for one or more fields", 400)
		return
	}

	row := t.db.QueryRow(`SELECT id, user_id, date, review, title,
                          instructor_id, class_id, recommend FROM review
                          WHERE id = ?`, id)

	review := new(Review)
	if err := row.Scan(
		&review.Id,
		&review.User_id,
		&review.Date,
		&review.Review,
		&review.Title,
		&review.Instructor_id,
		&review.Class_id,
		&review.Recommend); err != nil {
		log.Println("Review", err)
		ServeError(w, r, "Internal server error", 500)
		return
	}

	user, err := GetUserById(t.db, review.User_id)
	if err != nil {
		log.Println("GetReview", err)
		ServeError(w, r, "Internal server error", 500)
		return
	}
	review.User = user

	instructor, err := FetchInstructorById(t.db, review.Instructor_id)
	if err != nil {
		log.Println("GetReview", err)
		ServeError(w, r, "Internal server error", 500)
		return
	}
	review.Instructor = instructor

	comments, err := FetchCommentsByReviewId(t.db, review.Id)
	if err != nil {
		log.Println("GetReview", err)
		ServeError(w, r, "Internal server error", 500)
		return
	}
	review.Comments = comments

	ServeResult(w, r, review)
}
