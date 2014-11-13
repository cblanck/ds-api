package main

import (
	"database/sql"
	"time"
)

/*
 * Users
 */
type UserData struct {
	Id                 int64
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
func GetUserById(db *sql.DB, uid int64) (*UserData, error) {
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

/*
 * Classes
 */
type Class struct {
	Id               int64
	Subject          int64
	Subject_callsign string
	Course_number    int64
	Description      string
}

// Get the details of a class by ID
func GetClassById(db *sql.DB, id int64) (*Class, error) {
	row := db.QueryRow(`SELECT class.id, class.subject, subject.callsign,
    class.course_number, class.description FROM class, subject
    WHERE class.subject = subject.id
    AND class.id = ?`, id)
	class := new(Class)
	err := row.Scan(
		&class.Id,
		&class.Subject,
		&class.Subject_callsign,
		&class.Course_number,
		&class.Description,
	)
	return class, err
}

/*
 * Instructor
 */

type Instructor struct {
	Id    int64
	Name  string
	Email string
}

func GetInstructorById(db *sql.DB, id int64) (*Instructor, error) {
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

/*
 * Comments
 */

type Comment struct {
	Id        int64
	Review_id int64
	User_id   int64
	Date      time.Time
	Text      string
	User      *UserData
}

/*
 * Review
 */

type Review struct {
	Id            int64
	User_id       int64
	Date          time.Time
	Review        string
	Title         string
	Instructor_id int64
	Class_id      int64
	Recommend     bool
	User          *UserData
	Instructor    *Instructor
	Comments      []*Comment
}

func GetReviewById(db *sql.DB, id int64) (*Review, error) {
	row := db.QueryRow(`SELECT id, user_id, date, review, title,
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
		return nil, err
	}
	return review, nil
}

/*
 * Category of classes (e.g. HASS, etc)
 */

type DSCategory struct {
	Id    int
	Name  string
	rules []*DSCategoryRule
}

/*
 * A category rule
 */

type DSCategoryRule struct {
	Id               int
	Category         int
	Ruletype         int
	Class_id         sql.NullInt64
	Category_id      sql.NullInt64
	Inherited_id     sql.NullInt64
	Passfail_allowed sql.NullBool
}

// Get the details of a category by ID
func GetDSCategoryById(db *sql.DB, id int64) (*DSCategory, error) {
	category := new(DSCategory)
	err := db.QueryRow(
		"SELECT id, name FROM ds_category WHERE id = ?",
		id).Scan(&category.Id, &category.Name)
	if err != nil {
		return nil, err
	}
	category.rules, err = GetRulesForCategory(db, id)
	if err != nil {
		return nil, err
	}
	return category, nil
}

/*
 * The enum replacement for ruletype
 */

type DSCategoryRuleType struct {
	Id       int
	Ruletype string
}
