/*
 * API implemented by this servlet is to be in accordance with
 * http://red.degreesheep.com/projects/api/wiki/Spec
 */

package main

import (
	"code.google.com/p/go.crypto/pbkdf2"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"log"
	"math/rand"
	"net/http"
	"reflect"
	"strings"
	"time"
)

type UserServlet struct {
	db              *sql.DB
	random          *rand.Rand
	session_manager *SessionManager
}

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

func NewUserServlet(server_config Config, session_manager *SessionManager) *UserServlet {
	t := new(UserServlet)
	t.random = rand.New(rand.NewSource(time.Now().UnixNano()))

	t.session_manager = session_manager

	db, err := sql.Open("mysql", server_config.GetSqlURI())
	if err != nil {
		log.Fatal("NewUserServlet", "Failed to open database:", err)
	}
	t.db = db

	return t
}

// To avoid a massive case statement, use reflection to do a lookup of the given
// method on the servlet. MethodByName will return a 'Zero Value' for methods
// that aren't found, which will return false for .IsValid.
// Performing Call() on an unexported method is a runtime violation, uppercasing
// the first letter in the method name before reflection avoids locating
// unexported functions. A little hacky, but it works.
//
// For more info, see http://golang.org/pkg/reflect/
func (t *UserServlet) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	method := r.Form.Get("method")

	if method == "" {
		ServeError(w, r, "No method specified", 405)
		return
	}

	upper_method := strings.ToUpper(method)
	exported_method := []byte(method)
	exported_method[0] = upper_method[0]

	servlet_value := reflect.ValueOf(t)
	method_handler := servlet_value.MethodByName(string(exported_method))
	if method_handler.IsValid() {
		args := make([]reflect.Value, 2)
		args[0] = reflect.ValueOf(w)
		args[1] = reflect.ValueOf(r)
		method_handler.Call(args)
	} else {
		ServeError(w, r, fmt.Sprintf("No such method: %s", method), 405)
	}
}

func (t *UserServlet) CheckSession(w http.ResponseWriter, r *http.Request) {
	session_id := r.Form.Get("session")
	session_valid, session, err := t.session_manager.GetSession(session_id)
	if err != nil {
		log.Println("CheckSession", err)
		ServeError(w, r, "Internal Server Error", 500)
		return
	}
	if !session_valid {
		ServeError(w, r, fmt.Sprintf("Session has expired. Please log in again"), 200)
		return
	}
	ServeError(w, r, fmt.Sprintf("Got valid session for %s", session.user.First_name), 200)
}

// Create a login session for a user.
// Session tokens are stored in a local cache, as well as back to the DB to
// support multi-server architecture. A cache miss will result in a DB read.
func (t *UserServlet) Login(w http.ResponseWriter, r *http.Request) {
	user := r.Form.Get("user")
	pass := r.Form.Get("pass")

	rows, err := t.db.Query("SELECT password, password_salt FROM user WHERE username = ?", user)
	if err != nil {
		log.Println("Login", err)
		ServeError(w, r, fmt.Sprintf("Internal server error"), 500)
		return
	}

	defer rows.Close()
	var password_hash string
	var password_salt string
	for rows.Next() {
		if err := rows.Scan(&password_hash, &password_salt); err != nil {
			log.Println("Login", err)
			ServeError(w, r, fmt.Sprintf("Internal server error"), 500)
			return
		}
	}
	if err := rows.Err(); err != nil {
		log.Println("Login", err)
		ServeError(w, r, fmt.Sprintf("Internal server error"), 500)
		return
	}

	generated_hash := t.generate_password_hash([]byte(pass), []byte(password_salt))

	if string(generated_hash) == password_hash {
		// Successful login
		userdata, err := t.process_login(user)
		if err != nil {
			log.Println("process_login", err)
			ServeError(w, r, fmt.Sprintf("Internal server error"), 500)
			return
		} else {
			ServeResult(w, r, userdata)
		}
	} else {
		// Invalid username / password combination
		ServeError(w, r, "Invalid username and/or password", 200)
	}
}

func (t *UserServlet) process_login(user string) (*UserData, error) {
	userdata, err := FetchUserByName(t.db, user)
	if err != nil {
		return nil, err
	}
	userdata.Session_token, err = t.session_manager.CreateSessionForUser(userdata.Id)
	if err != nil {
		return nil, err
	}
	err = t.update_last_login_for_user(user)
	if err != nil {
		log.Println("process_login", err)
	}

	return userdata, nil
}

// Update the last_login field for a user
func (t *UserServlet) update_last_login_for_user(user string) error {
	_, err := t.db.Exec("UPDATE degreesheep.user SET last_login = CURRENT_TIMESTAMP() WHERE username = ?", user)
	return err
}

func FetchUserByName(db *sql.DB, username string) (*UserData, error) {
	rows, err := db.Query(`SELECT id, username, password, password_salt,
		email, first_name, last_name, class_year, account_created, last_login,
		password_reset_key FROM degreesheep.user WHERE username = ?`, username)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	num_rows := 0
	user_data := new(UserData)
	for rows.Next() {
		num_rows++
		if err := rows.Scan(
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
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if num_rows == 0 {
		return nil, errors.New(fmt.Sprintf("Failed to get data for %s - no such user", username))
	}
	return user_data, nil
}

// Get information for a user by UID
func FetchUserById(db *sql.DB, uid int) (*UserData, error) {
	rows, err := db.Query(`SELECT id, username, password, password_salt,
		email, first_name, last_name, class_year, account_created, last_login,
		password_reset_key FROM degreesheep.user WHERE id = ?`, uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	num_rows := 0
	user_data := new(UserData)
	for rows.Next() {
		num_rows++
		if err := rows.Scan(
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
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if num_rows == 0 {
		return nil, errors.New(fmt.Sprintf("Failed to get data for UID %d - no such user", uid))
	}
	return user_data, nil
}

// Create a new user, then allocate a new session.
func (t *UserServlet) Register(w http.ResponseWriter, r *http.Request) {
	user := r.Form.Get("user")
	pass := r.Form.Get("pass")
	email := r.Form.Get("email")
	firstname := r.Form.Get("firstname")
	lastname := r.Form.Get("lastname")
	classyear := r.Form.Get("classyear")

	// If any of the fields (other than classyear) are nil, error out.
	if user == "" || pass == "" || email == "" || firstname == "" || lastname == "" || classyear == "" {
		ServeError(w, r, "Missing value for one or more fields", 200)
		return
	}

	// Check if the username is already taken
	name_exists, err := t.username_exists(user)
	if err != nil {
		log.Println("Register", err)
		ServeError(w, r, fmt.Sprintf("Internal server error"), 500)
		return
	}
	if name_exists {
		ServeError(w, r, fmt.Sprintf("Username %s is already taken", user), 200)
		return
	}

	password_salt := t.generate_random_bytestring(64)
	password_hash := t.generate_password_hash([]byte(pass), password_salt)

	_, err = t.db.Exec(`INSERT INTO  degreesheep.user (
        username, password, password_salt, email, first_name,
        last_name, class_year ) VALUES ( ?, ?, ?, ?, ?, ?, ?)`, user, password_hash, password_salt, email, firstname, lastname, classyear)
	if err != nil {
		log.Println("Register", err)
		ServeError(w, r, fmt.Sprintf("Internal server error"), 500)
		return
	}

	// Log in as the new user
	userdata, err := t.process_login(user)
	if err != nil {
		log.Println("process_login", err)
		ServeError(w, r, fmt.Sprintf("Internal server error"), 500)
		return
	} else {
		ServeResult(w, r, userdata)
	}
}

// Forgot password action for users.
// Generates a new recovery token, and emails it to the user.
func (t *UserServlet) Forgot_password(w http.ResponseWriter, r *http.Request) {
	user := r.Form.Get("user")

	// Generate a recovery token and associate it with the account
	reset_token := t.generate_random_alphanumeric(32)
	_, err := t.db.Exec("UPDATE user SET password_reset_key = ? WHERE username = ?", reset_token, user)
	if err != nil {
		log.Println("Forgot_password", err)
		ServeError(w, r, fmt.Sprintf("Internal server error"), 500)
		return
	}
	ServeError(w, r, fmt.Sprintf("Not implemented"), 501)
}

// Processing a password reset. Reads the reset token, checks it against the DB,
// and if valid updates the user's salt and password.
// Returns a new session.
func (t *UserServlet) Reset_password(w http.ResponseWriter, r *http.Request) {
	user := r.Form.Get("user")
	reset_key := r.Form.Get("reset_key")
	new_pass := r.Form.Get("new_pass")

	// Fetch the user information, including password reset key
	user_data, err := FetchUserByName(t.db, user)
	if err != nil {
		log.Println("Reset_password", err)
		ServeError(w, r, fmt.Sprintf("Internal server error"), 500)
		return
	}

	// If the reset keys do not match, they cannot reset the password
	if user_data.password_reset_key != reset_key {
		ServeError(w, r, fmt.Sprintf("Invalid password reset key"), 200)
		return
	}

	// Generate a new salt and hash from the new password
	password_salt := t.generate_random_bytestring(64)
	password_hash := t.generate_password_hash([]byte(new_pass), password_salt)

	// Update the user
	t.db.Exec("UPDATE user SET password = ?, password_salt = ? WHERE username = ?",
		password_hash, password_salt, user,
	)

	userdata, err := t.process_login(user)
	if err != nil {
		log.Println("process_login", err)
		ServeError(w, r, fmt.Sprintf("Internal server error"), 500)
		return
	} else {
		ServeResult(w, r, userdata)
	}
}

// Check if a username already exists in the degreesheep DB.
// Returns an error if any database operation fails.
func (t *UserServlet) username_exists(user string) (bool, error) {
	rows, err := t.db.Query("SELECT id FROM user WHERE username = ?", user)
	if err != nil {
		return true, err
	}
	defer rows.Close()
	num_rows := 0
	for rows.Next() {
		num_rows++
		var id int
		if err := rows.Scan(&id); err != nil {
			return true, err
		}
	}
	if err := rows.Err(); err != nil {
		return true, err
	}
	if num_rows > 0 {
		return true, nil
	} else {
		return false, nil
	}
}

// Create a random bytestring
func (t *UserServlet) generate_random_bytestring(length int) []byte {
	random_bytes := make([]byte, length)
	for i := range random_bytes {
		random_bytes[i] = byte(t.random.Int() & 0xff)
	}
	return random_bytes
}

// Create a random bytestring
func (t *UserServlet) generate_random_alphanumeric(length int) []byte {
	random_bytes := make([]byte, length)
	for i := range random_bytes {
		random_bytes[i] = byte(t.random.Int() & 0xff)
	}
	return random_bytes
}

// Generate a PBKDF password hash. Use 4096 iterations and a 64 byte key.
func (t *UserServlet) generate_password_hash(password, salt []byte) []byte {
	return pbkdf2.Key(password, salt, 4096, 64, sha256.New)
}
