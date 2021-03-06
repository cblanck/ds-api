/*
 * API implemented by this servlet is to be in accordance with
 * http://red.degreesheep.com/projects/api/wiki/Spec
 */

package main

import (
	"bytes"
	"code.google.com/p/go.crypto/pbkdf2"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"log"
	"math/rand"
	"net/http"
	"time"
)

type UserServlet struct {
	db              *sql.DB
	random          *rand.Rand
	server_config   *Config
	session_manager *SessionManager
	email_manager   *EmailManager
}

const alphanumerics = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890"

func NewUserServlet(server_config *Config, session_manager *SessionManager, email_manager *EmailManager) *UserServlet {
	t := new(UserServlet)
	t.random = rand.New(rand.NewSource(time.Now().UnixNano()))

	t.session_manager = session_manager
	t.email_manager = email_manager
	t.server_config = server_config

	db, err := sql.Open("mysql", server_config.GetSqlURI())
	if err != nil {
		log.Fatal("NewUserServlet", "Failed to open database:", err)
	}
	t.db = db

	return t
}

func (t *UserServlet) Validate(r *http.Request) *ApiResult {
	session_id := r.Form.Get("session")
	session_valid, session, err := t.session_manager.GetSession(session_id)
	if err != nil {
		log.Println("Validate", err)
		return nil
	}
	if !session_valid {
		return nil
	}
	return APISuccess(session.User)
}

// Create a login session for a user.
// Session tokens are stored in a local cache, as well as back to the DB to
// support multi-server architecture. A cache miss will result in a DB read.
func (t *UserServlet) CacheableLogin(r *http.Request) *ApiResult {
	user := r.Form.Get("user")
	pass := r.Form.Get("pass")

	// Verify the password
	password_valid, err := t.verify_password_for_user(user, pass)
	if err != nil {
		log.Println("Login", err)
		return nil
	}

	if password_valid {
		// Successful login
		userdata, err := t.process_login(user)
		if err != nil {
			log.Println("process_login", err)
			return nil
		} else {
			return APISuccess(userdata)
		}
	} else {
		// Invalid username / password combination
		return APIError("Invalid username and/or password", 200)
	}
}

func (t *UserServlet) Get(r *http.Request) *ApiResult {
	session_id := r.Form.Get("session")
	session_valid, session, err := t.session_manager.GetSession(session_id)
	if err != nil {
		log.Println(err)
		return APIError("Internal Server Error", 500)
	}
	if !session_valid {
		return APIError("Session has expired. Please log in again", 200)
	}
	return APISuccess(session.User)
}

func (t *UserServlet) Delete(r *http.Request) *ApiResult {
	session_id := r.Form.Get("session")
	session_valid, session, err := t.session_manager.GetSession(session_id)
	if err != nil {
		log.Println(err)
		return APIError("Internal Server Error", 500)
	}
	if !session_valid {
		return APIError("Session has expired. Please log in again", 200)
	}

	_, err = t.db.Exec(`
		DELETE FROM degree_sheet, taken_courses
		WHERE taken_courses.sheet_id = degree_sheet.id
		AND degree_sheet.user_id = ?`, session.User.Id)
	if err != nil {
		log.Println(err)
		return APIError("Internal Server Error", 500)
	}

	_, err = t.db.Exec("DELETE FROM review WHERE user_id = ?", session.User.Id)
	if err != nil {
		log.Println(err)
		return APIError("Internal Server Error", 500)
	}

	_, err = t.db.Exec("DELETE FROM user where id = ?", session.User.Id)
	if err != nil {
		log.Println(err)
		return APIError("Internal Server Error", 500)
	}
	return APISuccess("OK")
}

func (t *UserServlet) Modify(r *http.Request) *ApiResult {
	session_id := r.Form.Get("session")
	session_valid, session, err := t.session_manager.GetSession(session_id)
	if err != nil {
		log.Println(err)
		return APIError("Internal Server Error", 500)
	}
	if !session_valid {
		return APIError("Session has expired. Please log in again", 200)
	}

	pass := r.Form.Get("pass")
	email := r.Form.Get("email")
	firstname := r.Form.Get("firstname")
	lastname := r.Form.Get("lastname")
	classyear := r.Form.Get("classyear")

	if pass != "" {
		t.set_password_for_user(session.User.Username, pass)
	}

	if email != "" {
		_, err := t.db.Exec("UPDATE user set email = ? WHERE id = ?", email, session.User.Id)
		if err != nil {
			return APIError("Failed to update email", 500)
		}
	}

	if firstname != "" {
		_, err := t.db.Exec("UPDATE user set first_name = ? WHERE id = ?", firstname, session.User.Id)
		if err != nil {
			return APIError("Failed to update firstname", 500)
		}
	}

	if lastname != "" {
		_, err := t.db.Exec("UPDATE user set last_name = ? WHERE id = ?", lastname, session.User.Id)
		if err != nil {
			return APIError("Failed to update last_name", 500)
		}
	}

	if classyear != "" {
		_, err := t.db.Exec("UPDATE user set class_year = ? WHERE id = ?", classyear, session.User.Id)
		if err != nil {
			return APIError("Failed to update classyear", 500)
		}
	}
	return APISuccess("OK")
}

// Verify a password for a username.
// Returns whether or not the password was valid and whether an error occurred.
func (t *UserServlet) verify_password_for_user(user, pass string) (bool, error) {
	rows, err := t.db.Query("SELECT password, password_salt FROM user WHERE username = ?", user)
	if err != nil {
		return false, err
	}

	defer rows.Close()
	var password_hash_base64 string
	var password_salt_base64 string
	for rows.Next() {
		if err := rows.Scan(&password_hash_base64, &password_salt_base64); err != nil {
			return false, err
		}
	}
	if err := rows.Err(); err != nil {
		return false, err
	}

	password_hash, err := base64.StdEncoding.DecodeString(password_hash_base64)
	if err != nil {
		return false, err
	}
	password_salt, err := base64.StdEncoding.DecodeString(password_salt_base64)
	if err != nil {
		return false, err
	}

	generated_hash := t.generate_password_hash([]byte(pass), []byte(password_salt))

	// Verify the byte arrays for equality. bytes.Compare returns 0 if the two
	// arrays are equivalent.
	if bytes.Compare(generated_hash, password_hash) == 0 {
		return true, nil
	} else {
		return false, nil
	}
}

// Fetches a user's data and creates a session for them.
// Returns a pointer to the userdata and an error.
func (t *UserServlet) process_login(user string) (*UserData, error) {
	userdata, err := GetUserByName(t.db, user)
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

// Create a new user, then allocate a new session.
func (t *UserServlet) Register(r *http.Request) *ApiResult {
	user := r.Form.Get("user")
	pass := r.Form.Get("pass")
	email := r.Form.Get("email")
	firstname := r.Form.Get("firstname")
	lastname := r.Form.Get("lastname")
	classyear := r.Form.Get("classyear")

	// If any of the fields (other than classyear) are nil, error out.
	if user == "" || pass == "" || email == "" || firstname == "" || lastname == "" || classyear == "" {
		return APIError("Missing value for one or more fields", 400)
	}

	// Check if the username is already taken
	name_exists, err := t.username_exists(user)
	if err != nil {
		log.Println("Register", err)
		return nil
	}
	if name_exists {
		return APIError(fmt.Sprintf("Username %s is already taken", user), 200)
	}

	// Create the user
	_, err = t.db.Exec(`INSERT INTO  degreesheep.user (
        username, email, first_name,
        last_name, class_year ) VALUES ( ?, ?, ?, ?, ?)`,
		user, email, firstname, lastname, classyear)
	if err != nil {
		log.Println("Register", err)
		return nil
	}

	// Set the password for the user
	t.set_password_for_user(user, pass)

	// Log in as the new user
	userdata, err := t.process_login(user)
	if err != nil {
		log.Println("process_login", err)
		return nil
	} else {
		return APISuccess(userdata)
	}
}

// Sets the password for a user by username.
// Generates a new salt as well.
// Values are stored as base64 encoded strings.
func (t *UserServlet) set_password_for_user(user, pass string) error {
	password_salt := t.generate_random_bytestring(64)
	password_hash := t.generate_password_hash([]byte(pass), password_salt)
	_, err := t.db.Exec("UPDATE degreesheep.user SET password = ?, password_salt = ? WHERE username = ?",
		base64.StdEncoding.EncodeToString(password_hash),
		base64.StdEncoding.EncodeToString(password_salt),
		user,
	)
	return err
}

// Forgot password action for users.
// Generates a new recovery token, and emails it to the user.
func (t *UserServlet) Forgot_password(r *http.Request) *ApiResult {
	user := r.Form.Get("user")

	// Generate a recovery token and associate it with the account
	reset_token := t.generate_random_alphanumeric(32)
	_, err := t.db.Exec("UPDATE user SET password_reset_key = ? WHERE username = ?", reset_token, user)
	if err != nil {
		log.Println("Forgot_password", err)
		return nil
	}

	user_data, err := GetUserByName(t.db, user)
	if err != nil {
		log.Println("Forgot_password", err)
		return nil
	}

	t.email_manager.QueueEmail(user_data.Email, t.server_config.Mail.From,
		"Password recovery for DegreeSheep",
		fmt.Sprintf(`Hey %s,
Someone (hopefully you) requested a password reset.
To change your password, click this link (or copy and paste it into your browser).
http://degreesheep.com/#/reset/%s/%s`, user_data.First_name, user, reset_token))

	return APISuccess("A password recovery email has been sent.")
}

// Processing a password reset. Reads the reset token, checks it against the DB,
// and if valid updates the user's salt and password.
// Returns a new session.
func (t *UserServlet) Reset_password(r *http.Request) *ApiResult {
	user := r.Form.Get("user")
	reset_key := r.Form.Get("reset_key")
	new_pass := r.Form.Get("new_pass")

	// Fetch the user information, including password reset key
	user_data, err := GetUserByName(t.db, user)
	if err != nil {
		log.Println("Reset_password", err)
		return nil
	}

	// If the reset keys do not match, they cannot reset the password
	if user_data.password_reset_key != reset_key {
		return APIError("Invalid password reset key", 200)
	}

	// Update the user
	t.set_password_for_user(user, new_pass)

	// Start a new session
	userdata, err := t.process_login(user)
	if err != nil {
		log.Println("process_login", err)
		return nil
	} else {
		return APISuccess(userdata)
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

// Create a random alphanumeric string
func (t *UserServlet) generate_random_alphanumeric(length int) []byte {
	random_bytes := make([]byte, length)

	for i := range random_bytes {
		random_bytes[i] = alphanumerics[t.random.Int()%len(alphanumerics)]
	}
	return random_bytes
}

// Generate a PBKDF password hash. Use 4096 iterations and a 64 byte key.
func (t *UserServlet) generate_password_hash(password, salt []byte) []byte {
	return pbkdf2.Key(password, salt, 4096, 64, sha256.New)
}
