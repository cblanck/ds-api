package main

import (
	"database/sql"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"log"
	"net/http"
	"strconv"
)

type DegreeSheetServlet struct {
	db              *sql.DB
	session_manager *SessionManager
}

func NewDegreeSheetServlet(server_config Config, session_manager *SessionManager) *DegreeSheetServlet {
	t := new(DegreeSheetServlet)
	t.session_manager = session_manager

	db, err := sql.Open("mysql", server_config.GetSqlURI())
	if err != nil {
		log.Fatal("NewUserServlet", "Failed to open database:", err)
	}
	t.db = db
	return t
}

// Remove a degree sheet entry by ID
func (t *DegreeSheetServlet) Remove_entry(r *http.Request) *ApiResult {
	// Validate the session
	session_uuid := r.Form.Get("session")
	session_valid, session, err := t.session_manager.GetSession(session_uuid)
	if err != nil {
		log.Println("Remove_entry", err)
		return APIError("Internal server error", 500)
	}
	if !session_valid {
		return APIError("The specified session has expired", 401)
	}

	// Grab the entry
	entry_id_s := r.Form.Get("entry_id")
	entry_id, err := strconv.ParseInt(entry_id_s, 10, 64)
	if err != nil {
		return APIError("Bad entry ID", 400)
	}
	entry, err := GetDegreeSheetEntryById(t.db, entry_id)
	if err != nil {
		log.Println("Remove_entry", err)
		return APIError("Internal server error", 500)
	}

	// Fetch the relevant DegreeSheet
	sheet, err := GetDegreeSheetById(t.db, entry.Sheet_id)
	if err != nil {
		log.Println("Remove_entry", err)
		return APIError("Internal server_error", 500)
	}

	// Verify that the logged in user owns the sheet
	if sheet.User_id != session.User.Id {
		return APIError(fmt.Sprintf("Sheet ID #%d is not owned by you", sheet.Id), 401)
	}

	// Drop the entry
	_, err = t.db.Exec(`DELETE FROM degree_sheet_entry WHERE id = ?`, entry_id)
	if err != nil {
		log.Println("Remove_entry", err)
		return APIError("Internal server error", 500)
	}
	return APISuccess("OK")
}

/* Adds a new DS entry to the specified degree sheet.
* Params:
- Valid session
- Degree sheet ID
- Class ID
- Year
- Semester
- Grade
- Whether it was pass/fail
*/
func (t *DegreeSheetServlet) Add_entry(r *http.Request) *ApiResult {
	// Validate the session
	session_uuid := r.Form.Get("session")
	session_valid, session, err := t.session_manager.GetSession(session_uuid)
	if err != nil {
		log.Println("Add_entry", err)
		return APIError("Internal server error", 500)
	}
	if !session_valid {
		return APIError("The specified session has expired", 401)
	}

	// Fetch the relevant DegreeSheet
	sheet_id_s := r.Form.Get("sheet_id")
	sheet_id, err := strconv.ParseInt(sheet_id_s, 10, 64)
	if err != nil {
		log.Println("Add_entry", err)
		return APIError("Internal server_error", 500)
	}
	sheet, err := GetDegreeSheetById(t.db, sheet_id)
	if err != nil {
		log.Println("Add_entry", err)
		return APIError("Internal server_error", 500)
	}

	// Verify that the logged in user owns the sheet
	if sheet.User_id != session.User.Id {
		return APIError(fmt.Sprintf("Sheet ID #%d is not owned by you", sheet_id), 401)
	}

	// Create the entry
	class_id := r.Form.Get("class_id")
	year := r.Form.Get("year")
	semester := r.Form.Get("semester")
	grade := r.Form.Get("grade")
	passfail := r.Form.Get("passfail")

	_, err = t.db.Exec(`INSERT INTO degree_sheet_entry (
        sheet_id, class_id, year, semester, grade, passfail
    ) VALUES (
        ?, ?, ?, ?, ?, ?
    )`, sheet.Id, class_id, year, semester, grade, passfail)

	if err != nil {
		log.Println("Add_entry", err)
		return APIError("Internal server error", 500)
	}

	return APISuccess("OK")
}

func (t *DegreeSheetServlet) List_sheets(r *http.Request) *ApiResult {
	session_id := r.Form.Get("session")
	session_valid, session, err := t.session_manager.GetSession(session_id)

	if err != nil {
		log.Println("List_sheets", err)
		return APIError("Internal server error", 500)
	}
	if !session_valid {
		return APIError("The specified session has expired", 401)
	}

	rows, err := t.db.Query(`
		SELECT degree_sheet.id, degree_sheet.created, degree_sheet.name,
			degree_sheet.template_id, ds_category.name
		FROM degree_sheet, ds_category
		WHERE ds_category.id = degree_sheet.template_id
		AND user_id = ?`, session.User.Id)
	if err != nil {
		log.Println("List_sheets", err)
		return APIError("Internal server error", 500)
	}

	defer rows.Close()
	sheet_list := make([]*DegreeSheet, 0)
	for rows.Next() {
		sheet := new(DegreeSheet)
		if err := rows.Scan(
			&sheet.Id,
			&sheet.Created,
			&sheet.Name,
			&sheet.Template_id,
			&sheet.Template_name); err != nil {
			log.Println("List_sheets", err)
			return APIError("Internal server error", 500)
		}
		sheet_list = append(sheet_list, sheet)
	}
	return APISuccess(sheet_list)
}

func (t *DegreeSheetServlet) Get_entries(r *http.Request) *ApiResult {
	session_id := r.Form.Get("session")
	session_valid, session, err := t.session_manager.GetSession(session_id)

	if err != nil {
		log.Println("Get_entries", err)
		return APIError("Internal server error", 500)
	}
	if !session_valid {
		return APIError("The specified session has expired", 401)
	}

	entry_list := make([]*DegreeSheetEntry, 0)

	sheet_id_s := r.Form.Get("sheet_id")
	sheet_id, err := strconv.ParseInt(sheet_id_s, 10, 64)
	if err != nil {
		return APIError("Bad sheet ID", 400)
	}
	sheet, err := GetDegreeSheetById(t.db, sheet_id)
	if err != nil {
		if err == sql.ErrNoRows {
			return APISuccess(entry_list)
		} else {
			log.Println("Get_entries", err)
			return APIError("Internal server error", 500)
		}
	}
	if sheet.User_id != session.User.Id {
		log.Println("Get_entries", err)
		return APIError("Unauthorized", 401)
	}

	rows, err := t.db.Query(
		`SELECT id, sheet_id, class_id, year, semester, grade, passfail
         FROM degree_sheet_entry WHERE sheet_id = ?`,
		sheet_id)

	if err != nil {
		if err == sql.ErrNoRows {
			return APISuccess(entry_list)
		} else {
			log.Println("Get_entries", err)
			return APIError("Internal server error", 500)
		}
	}

	defer rows.Close()
	for rows.Next() {
		entry := new(DegreeSheetEntry)
		if err := rows.Scan(
			&entry.Id,
			&entry.Sheet_id,
			&entry.Class_id,
			&entry.Year,
			&entry.Semester,
			&entry.Grade,
			&entry.Passfail); err != nil {
			log.Println("Get_entries", err)
			return APIError("Internal server error", 500)
		}
		entry.Class, err = GetClassById(t.db, entry.Class_id)
		if err != nil {
			log.Println("Get_entries", err)
			return APIError("Internal server error", 500)
		}
		entry_list = append(entry_list, entry)
	}
	return APISuccess(entry_list)
}

func (t *DegreeSheetServlet) Edit_entry(r *http.Request) *ApiResult {
	class_id := r.Form.Get("class_id")
	year := r.Form.Get("year")
	semester := r.Form.Get("semester")
	grade := r.Form.Get("grade")
	passfail := r.Form.Get("passfail")
	if class_id == "" || year == "" || semester == "" ||
		grade == "" || passfail == "" {
		return APIError("Missing value for one or more fields", 400)
	}
	session_id := r.Form.Get("session")
	session_valid, session, err := t.session_manager.GetSession(session_id)

	if err != nil {
		log.Println("Edit_entry", err)
		return APIError("Internal server error", 500)
	}
	if !session_valid {
		return APIError("The specified session has expired", 401)
	}
	entry_id_s := r.Form.Get("entry_id")
	entry_id, err := strconv.ParseInt(entry_id_s, 10, 64)
	if err != nil {
		return APIError("Bad entry ID", 400)
	}
	entry, err := GetDegreeSheetEntryById(t.db, entry_id)
	if err != nil {
		log.Println("Edit_entry", err)
		return APIError("Internal server error", 500)
	}
	sheet, err := GetDegreeSheetById(t.db, entry.Sheet_id)
	if err != nil {
		log.Println("Edit_entry", err)
		return APIError("Internal server error", 500)
	}
	if sheet.User_id != session.User.Id {
		log.Println("Edit_entry", err)
		return APIError("Unauthorized", 401)
	}

	_, err = t.db.Exec(
		`UPDATE degree_sheet_entry
         SET class_id = ?, year = ?, semester = ?, grade = ?, passfail = ?
         WHERE id = ?`, class_id, year, semester, grade, passfail, entry_id)
	if err != nil {
		log.Println("Edit_entry", err)
		return APIError("Internal server error", 500)
	}
	return APISuccess("OK")
}

func (t *DegreeSheetServlet) Add_sheet(r *http.Request) *ApiResult {
	session_id := r.Form.Get("session")
	session_valid, session, err := t.session_manager.GetSession(session_id)
	if err != nil {
		log.Println("Add_sheet", err)
		return APIError("Internal server error", 500)
	}
	if !session_valid {
		return APIError("The specified session has expired", 401)
	}
	name := r.Form.Get("name")
	template_id := r.Form.Get("template_id")

	if name == "" || template_id == "" {
		log.Println("Add_sheet", err)
		return APIError("Missing value for one or more fields", 400)
	}

	_, err = t.db.Exec(`INSERT INTO degree_sheet (user_id, template_id, name)
                     VALUES (?, ?, ?)`, session.User.Id, template_id, name)
	if err != nil {
		log.Println("Add_sheet", err)
		return APIError("Internal server error", 500)
	}
	return APISuccess("OK")
}

func (t *DegreeSheetServlet) Get_sheet(r *http.Request) *ApiResult {
	session_id := r.Form.Get("session")
	session_valid, session, err := t.session_manager.GetSession(session_id)
	if err != nil {
		log.Println("Add_sheet", err)
		return APIError("Internal server error", 500)
	}
	if !session_valid {
		return APIError("The specified session has expired", 401)
	}

	sheet_id_s := r.Form.Get("sheet_id")
	sheet_id, err := strconv.ParseInt(sheet_id_s, 10, 64)
	if err != nil {
		return APIError("Bad sheet ID", 400)
	}

	degree_sheet, err := GetDegreeSheetById(t.db, sheet_id)
	if degree_sheet.User_id != session.User.Id {
		return APIError("Specified sheet is not owned by you", 401)
	}

	return APISuccess(degree_sheet)
}

func (t *DegreeSheetServlet) Remove_sheet(r *http.Request) *ApiResult {
	session_id := r.Form.Get("session")
	session_valid, session, err := t.session_manager.GetSession(session_id)
	if err != nil {
		log.Println("Remove_sheet", err)
		return APIError("Internal server error", 500)
	}
	if !session_valid {
		return APIError("The specified session has expired", 401)
	}
	sheet_id_s := r.Form.Get("sheet_id")
	sheet_id, err := strconv.ParseInt(sheet_id_s, 10, 64)
	if err != nil {
		return APIError("Bad sheet ID", 400)
	}
	sheet, err := GetDegreeSheetById(t.db, sheet_id)
	if err != nil {
		log.Println("Remove_sheet", err)
		return APIError("Internal server error", 500)
	}
	if sheet.User_id != session.User.Id {
		log.Println("Remove_sheet", err)
		return APIError("Unauthorized", 401)
	}
	_, err = t.db.Exec("DELETE FROM degree_sheet WHERE id = ?", sheet_id)
	if err != nil {
		log.Println("Remove_sheet", err)
		return APIError("Internal server error", 500)
	}
	_, err = t.db.Exec("DELETE FROM degree_sheet_entries WHERE sheet_id = ?",
		sheet_id)
	if err != nil {
		log.Println("Remove_sheet", err)
		return APIError("Internal server error", 500)
	}
	return APISuccess("OK")
}

func (t *DegreeSheetServlet) Get_requirements_for_template(r *http.Request) *ApiResult {
	template_id_s := r.Form.Get("template_id")
	template_id, err := strconv.ParseInt(template_id_s, 10, 64)
	if err != nil {
		return APIError("Bad template ID", 400)
	}
	degree_template, err := GetDSCategoryById(t.db, template_id)
	if err != nil {
		log.Println(err)
		return APIError("Internal Server Error", 500)
	}
	return APISuccess(degree_template)
}
