package main

import (
	"database/sql"
	_ "github.com/go-sql-driver/mysql"
	"log"
	"net/http"
	"strconv"
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

func (t *ReviewServlet) List_reviews(r *http.Request) *ApiResult {
	class_id_s := r.Form.Get("class_id")

	if class_id_s == "" {
		return APIError("Missing value for one or more fields", 400)
	}

	class_id, err := strconv.ParseInt(class_id_s, 10, 64)
	if err != nil {
		return APIError("Invalid class ID", 400)
	}

	review_list, err := GetReviewsForClass(t.db, class_id)
	if err != nil {
		log.Println(err)
		return APIError("Internal server error", 500)
	}

	return APISuccess(review_list)
}

func (t *ReviewServlet) Post_review(r *http.Request) *ApiResult {
	session_id := r.Form.Get("session")

	session_valid, session, err := t.session_manager.GetSession(session_id)
	if err != nil {
		log.Println("Post_review", err)
		return APIError("Internal server error", 500)
	}
	if !session_valid {
		log.Println("Post_review", err)
		return APIError("The specified session has expired", 401)
	}

	review := r.Form.Get("review")
	title := r.Form.Get("title")
	instructor_id := r.Form.Get("instructor_id")
	class_id := r.Form.Get("class_id")
	recommend := r.Form.Get("recommend")

	if review == "" || title == "" || instructor_id == "" || class_id == "" ||
		recommend == "" {
		return APIError("Missing value for one or more fields", 400)
	}

	_, err = t.db.Exec(`INSERT INTO review (user_id, review, title,
                         instructor_id, class_id, recommend) VALUES (?, ?, ?, ?,?, ?)`,
		session.User.Id, review, title, instructor_id, class_id, recommend)
	if err != nil {
		log.Println("Post_review", err)
		return APIError("Internal server error", 500)
	}

	return APISuccess("OK")
}

func (t *ReviewServlet) Post_comment(r *http.Request) *ApiResult {
	session_id := r.Form.Get("session")

	session_valid, session, err := t.session_manager.GetSession(session_id)
	if err != nil {
		log.Println("Post_comment", err)
		return APIError("Internal server error", 500)
	}
	if !session_valid {
		log.Println("Post_comment", err)
		return APIError("The specified session has expired", 401)
	}

	review_id := r.Form.Get("review_id")
	text := r.Form.Get("text")

	if review_id == "" || text == "" {
		return APIError("Missing value for one or more fields", 400)
	}

	_, err = t.db.Exec(`INSERT INTO comment (review_id, user_id, text) VALUES
                        (?, ?, ?)`, review_id, session.User.Id, text)
	if err != nil {
		log.Println("Post_comment", err)
		return APIError("Internal server error", 500)
	}

	return APISuccess("OK")
}

func GetCommentsByReviewId(db *sql.DB, id int64) ([]*Comment, error) {
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

func (t *ReviewServlet) Get_review(r *http.Request) *ApiResult {
	id := r.Form.Get("review_id")

	if id == "" {
		return APIError("Missing value for one or more fields", 400)
	}

	review_id, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		log.Println("Get_review: ParseInt:", err)
		return APIError("Invalid review ID", 400)
	}
	review, err := GetReviewById(t.db, review_id)
	if err != nil {
		log.Println("Get_review: GetReviewById:", err)
		return APIError("Internal server error", 500)
	}

	user, err := GetUserById(t.db, review.User_id)
	if err != nil {
		log.Println("Get_review: GetUserById:", err)
		return APIError("Internal server error", 500)
	}
	review.User = user

	instructor, err := GetInstructorById(t.db, review.Instructor_id)
	if err != nil {
		log.Println("Get_review: GetInstructorById:", err)
		return APIError("Internal server error", 500)
	}
	review.Instructor = instructor

	comments, err := GetCommentsByReviewId(t.db, review.Id)
	if err != nil {
		log.Println("Get_review: FetchCommentsByReviewId:", err)
		return APIError("Internal server error", 500)
	}
	review.Comments = comments

	return APISuccess(review)
}
