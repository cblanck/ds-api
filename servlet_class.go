/*
 * API implemented by this servlet is to be in accordance with
 * http://red.degreesheep.com/projects/api/wiki/Spec
 */

package main

import (
	"database/sql"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"log"
	"net/http"
	"strconv"
)

type ClassServlet struct {
	db              *sql.DB
	server_config   *Config
	session_manager *SessionManager
}

func NewClassServlet(server_config *Config, session_manager *SessionManager) *ClassServlet {
	t := new(ClassServlet)

	t.session_manager = session_manager
	t.server_config = server_config

	db, err := sql.Open("mysql", server_config.GetSqlURI())
	if err != nil {
		log.Fatal("NewUserServlet", "Failed to open database:", err)
	}
	t.db = db

	return t
}

// Takes a class ID, returns the categories that the class can be used to fulfil
func (t *ClassServlet) CacheableMatched_categories(r *http.Request) *ApiResult {
	class_id_str := r.Form.Get("class_id")
	if class_id_str == "" {
		return APIError("Missing class_id", 400)
	}
	class_id, err := strconv.ParseInt(class_id_str, 10, 64)
	if err != nil {
		log.Println("Matched_categories", err)
		return APIError("Internal server error", 500)
	}
	categories, err := GetCategoriesMatchedbyClass(t.db, class_id)
	if err != nil {
		log.Println("Matched_categories", err)
		return APIError("Internal server error", 500)
	}
	return APISuccess(categories)
}

// Get a list of all classes we know
func (t *ClassServlet) CacheableList(r *http.Request) *ApiResult {
	class_list, err := get_all_classes(t.db)
	if err != nil {
		log.Println(err)
		return APIError("Internal server error", 500)
	}
	return APISuccess(class_list)
}

// Return the information for a single class
func (t *ClassServlet) CacheableGet(r *http.Request) *ApiResult {
	id_s := r.Form.Get("class_id")
	id, err := strconv.ParseInt(id_s, 10, 64)
	if err != nil {
		log.Println("Class.Get:", err)
		return APIError("Internal server error", 500)
	}
	c, err := GetClassById(t.db, id)
	if err != nil {
		log.Println("Class.Get:", err)
		return APIError("Internal server error", 500)
	}
	return APISuccess(c)
}

// Takes a variable number of constraints and outputs a list of classes that
// match all of those constraints.
func (t *ClassServlet) CacheableSearch(r *http.Request) *ApiResult {
	// Validate the session
	session_id := r.Form.Get("session")
	session_valid, _, err := t.session_manager.GetSession(session_id)
	if err != nil {
		log.Println("Search", err)
		return APIError(fmt.Sprintf("Internal server error"), 500)
	}
	if !session_valid {
		log.Println("Search", err)
		return APIError(fmt.Sprintf("The specified session has expired"), 401)
	}

	// Create a slice of class maps.
	// For each constraint, get a list of classes that satisfy those constraints
	class_maps := make([]map[int64]*Class, 0)
	var matching_classes []*Class = nil

	callsign := r.Form.Get("callsign")
	class_number := r.Form.Get("classnum")
	class_name := r.Form.Get("classname")
	rule := r.Form.Get("rule")

	if callsign != "" {
		classes, err := get_classes_by_callsign(t.db, callsign)
		if err != nil {
			log.Println("get_classes_by_callsign:", err)
			goto server_error
		}
		class_maps = append(class_maps, classes)
	}

	if class_number != "" {
		classnum, err := strconv.ParseInt(class_number, 10, 64)
		if err != nil {
			log.Println("get_classes_by_number:", err)
			goto server_error
		}

		classes, err := get_classes_by_number(t.db, classnum)
		if err != nil {
			log.Println("get_classes_by_number:", err)
			goto server_error
		}
		class_maps = append(class_maps, classes)
	}

	if class_name != "" {
		classes, err := get_classes_by_name(t.db, class_name)
		if err != nil {
			log.Println("get_classes_by_name:", err)
			goto server_error
		}
		class_maps = append(class_maps, classes)
	}

	if rule != "" {
		rule_id, err := strconv.ParseInt(rule, 10, 64)
		if err != nil {
			log.Println("get_classes_by_category:", err)
			goto server_error
		}
		classes, err := GetClassesForCategoryById(t.db, rule_id)
		if err != nil {
			log.Println("GetClassesForCategoryById", err)
			goto server_error
		}
		class_maps = append(class_maps, classes)
	}

	// Take the slice of maps and get a list of classes common to all maps
	matching_classes = get_common_classes(class_maps)

	return APISuccess(matching_classes)

server_error:
	return APIError("Internal server error", 500)
}

// Takes a slice of maps of class_id -> class and returns a list of classes that
// are common to all maps.
func get_common_classes(class_maps []map[int64]*Class) []*Class {
	common_classes := make([]*Class, 0)

	// If we didn't get any maps, return nothing.
	if len(class_maps) == 0 {
		return common_classes
	}
	for class_id, class := range class_maps[0] {
		class_is_common := true
		for _, class_map := range class_maps {
			_, exists := class_map[class_id]
			if !exists {
				class_is_common = false
				break
			}
		}
		if class_is_common {
			common_classes = append(common_classes, class)
		}
	}
	return common_classes
}

// Get a list of all classes in the DB
func get_all_classes(db *sql.DB) ([]*Class, error) {
	rows, err := db.Query(`SELECT class.id, class.subject, subject.callsign,
	subject.description, class.course_number, classn.name, class.description FROM class, subject
    WHERE class.subject = subject.id`)

	if err != nil {
		return nil, err
	}
	defer rows.Close()
	class_list := make([]*Class, 0)
	for rows.Next() {
		class := new(Class)
		if err := rows.Scan(
			&class.Id,
			&class.Subject_id,
			&class.Subject_callsign,
			&class.Subject_description,
			&class.Course_number,
			&class.Name,
			&class.Description); err != nil {
			return nil, err
		}
		class_list = append(class_list, class)
	}
	return class_list, nil
}

// Get a map of classid -> class for classes with a given callsign
func get_classes_by_callsign(db *sql.DB, callsign string) (map[int64]*Class, error) {
	rows, err := db.Query(`SELECT class.id, class.subject, subject.callsign,
    class.course_number, class.name, class.description FROM class, subject
    WHERE class.subject = subject.id
    AND subject.callsign LIKE ?`, callsign)

	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scan_class_rows(rows)
}

// Get a map of classid -> class for classes with a given callsign
func get_classes_by_number(db *sql.DB, classnum int64) (map[int64]*Class, error) {
	rows, err := db.Query(`SELECT class.id, class.subject, subject.callsign,
    class.course_number, class.name, class.description FROM class, subject
    WHERE class.subject = subject.id
    AND class.course_number = ?`, classnum)

	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scan_class_rows(rows)
}

// Get a map of classid -> class for classes with a matching name
func get_classes_by_name(db *sql.DB, name string) (map[int64]*Class, error) {
	rows, err := db.Query(`SELECT class.id, class.subject, subject.callsign,
    class.course_number, class.name, class.description FROM class, subject
    WHERE class.subject = subject.id
    AND class.name LIKE CONCAT(CONCAT('%',?),'%')`, name)

	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scan_class_rows(rows)
}

// Convenience function the deals with loading a series of rows of classes into
// a slice. Returns an error if the scan fails.
func scan_class_rows(rows *sql.Rows) (map[int64]*Class, error) {
	classes := make(map[int64]*Class)

	for rows.Next() {
		class := new(Class)
		if err := rows.Scan(
			&class.Id,
			&class.Subject_id,
			&class.Subject_callsign,
			&class.Subject_description,
			&class.Course_number,
			&class.Name,
			&class.Description); err != nil {
			return nil, err
		}
		classes[class.Id] = class
	}

	return classes, nil
}
