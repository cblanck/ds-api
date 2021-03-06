package main

import (
	"database/sql"
	"errors"
	"fmt"
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
	Id                  int64
	Subject_id          int64
	Subject_callsign    string
	Subject_description string
	Course_number       int64
	Name                string
	Description         string
	Instructors         []*Instructor
}

// Get the details of a class by ID
func GetClassById(db *sql.DB, id int64) (*Class, error) {
	row := db.QueryRow(`SELECT class.id, class.subject, subject.callsign,
	subject.description, class.course_number, class.name, class.description FROM class,
	subject WHERE class.subject = subject.id
    AND class.id = ?`, id)
	class := new(Class)
	err := row.Scan(
		&class.Id,
		&class.Subject_id,
		&class.Subject_callsign,
		&class.Subject_description,
		&class.Course_number,
		&class.Name,
		&class.Description,
	)
	if err != nil {
		return nil, err
	}
	class.Instructors, err = GetInstructorsForClass(db, class.Id)
	return class, err
}

/*
 * Category of classes (e.g. HASS, etc)
 */

type ClassCategory struct {
	Id      int
	Name    string
	Classes []*Class
}

func GetClassCategoryById(db *sql.DB, id int64) (*ClassCategory, error) {
	row := db.QueryRow(`SELECT class_category.id, class_category.name
	FROM class_category WHERE id = ?`, id)
	class_category := new(ClassCategory)
	err := row.Scan(
		&class_category.Id,
		&class_category.Name,
	)
	if err != nil {
		return nil, err
	}

	// Get the classes that the category matches
	rows, err := db.Query(`SELECT class_id
	FROM class_category_rule WHERE category = ?`, class_category.Id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	class_category.Classes = make([]*Class, 0)
	var class_id int64
	for rows.Next() {
		if err := rows.Scan(
			&class_id,
		); err != nil {
			return nil, err
		}
		class, err := GetClassById(db, class_id)
		if err != nil {
			return nil, err
		}
		class_category.Classes = append(class_category.Classes, class)
	}
	return class_category, nil
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

func GetInstructorsForClass(db *sql.DB, class_id int64) ([]*Instructor, error) {
	instructors := make([]*Instructor, 0)
	rows, err := db.Query(`
		SELECT DISTINCT(instructor.id), instructor.name,
		instructor.email FROM class,class_section,instructor
		WHERE class.id = class_section.class_id AND
		class_section.instructor_id = instructor.id AND class.id = ?`,
		class_id,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		i := new(Instructor)
		if err := rows.Scan(
			&i.Id,
			&i.Name,
			&i.Email,
		); err != nil {
			return nil, err
		}
		instructors = append(instructors, i)
	}
	return instructors, nil
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

func GetReviewsForClass(db *sql.DB, class_id int64) ([]*Review, error) {
	rows, err := db.Query(`SELECT id, user_id, date, review, title,
							instructor_id, class_id, recommend
							FROM review WHERE class_id = ?`, class_id)
	if err != nil {
		return nil, err
	}

	defer rows.Close()
	review_list := make([]*Review, 0)
	for rows.Next() {
		review := new(Review)
		if err := rows.Scan(
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
		review_list = append(review_list, review)
	}
	return review_list, nil
}

/*
 * A Degree sheet category (E.G. BSCS 2015 Foundation)
 */

type DSCategory struct {
	Id         int64
	Name       string
	Inherits   []*DSCategory
	Classes    []*Class
	Categories []*ClassCategory
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
	category.Inherits = make([]*DSCategory, 0)
	category.Classes = make([]*Class, 0)
	category.Categories = make([]*ClassCategory, 0)

	err = loadRulesForCategory(db, category)
	if err != nil {
		return nil, err
	}
	return category, nil
}

/*
 * A category rule
 */

type DSCategoryRule struct {
	Id          int64
	Category    int64
	Ruletype    int64
	Class_id    sql.NullInt64
	Category_id sql.NullInt64
	Inherit_id  sql.NullInt64
}

// Load in the rules for a category that has been instantiated with an ID
func loadRulesForCategory(db *sql.DB, category *DSCategory) error {
	rows, err := db.Query(`SELECT id, category, ruletype, class_id, category_id,
    inherited_id FROM ds_category_rule WHERE category = ?`, category.Id)
	if err != nil {
		return err
	}
	defer rows.Close()

	var rule DSCategoryRule
	for rows.Next() {
		if err := rows.Scan(
			&rule.Id,
			&rule.Category,
			&rule.Ruletype,
			&rule.Class_id,
			&rule.Category_id,
			&rule.Inherit_id,
		); err != nil {
			return err
		}
		if rule.Ruletype == RULE_CLASS {
			if rule.Class_id.Valid {
				class, err := GetClassById(db, rule.Class_id.Int64)
				if err != nil {
					return err
				}
				category.Classes = append(category.Classes, class)
			} else {
				return errors.New(fmt.Sprintf("Malformed DSCategory rule #%d", rule.Id))
			}
		} else if rule.Ruletype == RULE_CATEGORY {
			if rule.Category_id.Valid {
				class_cat, err := GetClassCategoryById(db, rule.Category_id.Int64)
				if err != nil {
					return err
				}
				category.Categories = append(category.Categories, class_cat)
			} else {
				return errors.New(fmt.Sprintf("Malformed DSCategory rule #%d", rule.Id))
			}
		} else if rule.Ruletype == RULE_INHERIT {
			if rule.Inherit_id.Valid {
				ds_cat, err := GetDSCategoryById(db, rule.Inherit_id.Int64)
				if err != nil {
					return err
				}
				category.Inherits = append(category.Inherits, ds_cat)
			} else {
				return errors.New(fmt.Sprintf("Malformed DSCategory rule #%d", rule.Id))
			}
		}
	}
	return nil
}

/*
 * The enum replacement for ruletype
 */

type DSCategoryRuleType struct {
	Id       int64
	Ruletype string
}

/*
 * Representation of a single degree sheet
 */

type DegreeSheet struct {
	Id              int64
	Created         time.Time
	User_id         int64
	Template_id     int64
	Template_name   string
	Name            string
	Taken_courses   []*TakenCourse
	Planned_courses []*PlannedClass
	Dropped_courses SatisfactionMap
}

func GetDegreeSheetById(db *sql.DB, id int64) (*DegreeSheet, error) {
	sheet := new(DegreeSheet)
	err := db.QueryRow(
		`SELECT degree_sheet.id, degree_sheet.Created, degree_sheet.user_id,
				degree_sheet.template_id, degree_sheet.name, ds_category.name
		FROM degree_sheet, ds_category
		WHERE degree_sheet.template_id = ds_category.id AND degree_sheet.id = ?`,
		id).Scan(
		&sheet.Id,
		&sheet.Created,
		&sheet.User_id,
		&sheet.Template_id,
		&sheet.Name,
		&sheet.Template_name,
	)
	if err != nil {
		return nil, err
	}

	sheet.Taken_courses, err = GetTakenCoursesForUser(db, sheet.User_id)
	if err != nil {
		return nil, err
	}

	sheet.Planned_courses, err = GetPlannedClassesForUser(db, sheet.User_id)
	if err != nil {
		return nil, err
	}

	sheet.Dropped_courses, err = GetSavedStateForSheet(db, sheet.Id)
	if err != nil {
		return nil, err
	}

	return sheet, nil
}

type SatisfactionMap map[string]int64

func GetSavedStateForSheet(db *sql.DB, sheet_id int64) (SatisfactionMap, error) {
	rows, err := db.Query(
		"SELECT requirement_id, satisfier_id FROM degree_sheet_entry WHERE sheet_id = ?",
		sheet_id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entries := make(map[string]int64, 0)
	for rows.Next() {
		var req string
		var sat int64
		if err := rows.Scan(
			&req,
			&sat); err != nil {
			return nil, err
		}
		entries[req] = sat
	}
	return entries, nil
}

/*
 * Courses taken by a student
 */

type TakenCourse struct {
	Id       int64
	User_id  int64
	Class_id int64
	Class    *Class
	Year     int64
	Semester int64
	Grade    string
	Passfail bool
}

func GetTakenCoursesForUser(db *sql.DB, user_id int64) ([]*TakenCourse, error) {
	rows, err := db.Query(
		"SELECT id, class_id, year, semester, grade, passfail FROM taken_courses WHERE user_id = ?",
		user_id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entries := make([]*TakenCourse, 0)
	for rows.Next() {
		entry := new(TakenCourse)
		if err := rows.Scan(
			&entry.Id,
			&entry.Class_id,
			&entry.Year,
			&entry.Semester,
			&entry.Grade,
			&entry.Passfail); err != nil {
			return nil, err
		}
		entry.Class, _ = GetClassById(db, entry.Class_id)
		entries = append(entries, entry)
	}
	return entries, nil
}

/*
 * Planned classes
 */
type PlannedClass struct {
	Id       int64
	Added    time.Time
	Class_id int64
	Class    *Class
}

func GetPlannedClassesForUser(db *sql.DB, user_id int64) ([]*PlannedClass, error) {
	rows, err := db.Query(
		`SELECT id, added, class_id FROM planned_class WHERE user_id = ?`,
		user_id,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	planned_classes := make([]*PlannedClass, 0)
	for rows.Next() {
		p_c := new(PlannedClass)
		if err := rows.Scan(
			&p_c.Id,
			&p_c.Added,
			&p_c.Class_id); err != nil {
			return nil, err
		}
		p_c.Class, _ = GetClassById(db, p_c.Class_id)
		planned_classes = append(planned_classes, p_c)
	}

	return planned_classes, nil
}

func AddPlannedClassForUser(db *sql.DB, user_id int64, class_id int64) error {
	_, err := db.Exec(
		`INSERT INTO planned_class (added, user_id, class_id)
		 VALUES (CURRENT_TIMESTAMP(), ?, ?)
		 ON DUPLICATE KEY UPDATE added = CURRENT_TIMESTAMP()`,
		user_id, class_id)
	return err
}

func DeletePlannedClassForUser(db *sql.DB, class_id int64, user_id int64) error {
	_, err := db.Exec(
		`DELETE FROM planned_class WHERE class_id = ? AND user_id = ?`,
		class_id, user_id)
	return err
}

func GetTakenCourseById(db *sql.DB, id int64) (*TakenCourse, error) {
	entry := new(TakenCourse)
	err := db.QueryRow(
		"SELECT id, user_id, class_id, year, semester, grade, passfail FROM taken_courses WHERE id = ?",
		id).Scan(
		&entry.Id,
		&entry.User_id,
		&entry.Class_id,
		&entry.Year,
		&entry.Semester,
		&entry.Grade,
		&entry.Passfail)
	if err != nil {
		return nil, err
	}
	return entry, nil
}
