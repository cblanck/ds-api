/*
 * API implemented by this servlet is to be in accordance with
 * http://red.degreesheep.com/projects/api/wiki/Spec
 */

package main

import (
	"code.google.com/p/go.crypto/pbkdf2"
	"crypto/sha256"
	"database/sql"
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
	db     *sql.DB
	random *rand.Rand
}

func NewUserServlet(server_config Config) *UserServlet {
	t := new(UserServlet)
	t.random = rand.New(rand.NewSource(time.Now().UnixNano()))

	db, err := sql.Open("mysql", server_config.GetSqlURI())
	if err != nil {
		log.Fatal("Failed to open database:", err)
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

// Create a login session for a user.
// Session tokens are stored in a local cache, as well as back to the DB to
// support multi-server architecture. A cache miss will result in a DB read.
func (t *UserServlet) Login(w http.ResponseWriter, r *http.Request) {
	user := r.Form.Get("user")
	pass := r.Form.Get("pass")

	rows, err := t.db.Query("SELECT password, password_salt FROM user WHERE username = ?", user)
	if err != nil {
		log.Println(err)
		ServeError(w, r, fmt.Sprintf("Internal server error"), 500)
		return
	}

	defer rows.Close()
	var password_hash string
	var password_salt string
	for rows.Next() {
		if err := rows.Scan(&password_hash, &password_salt); err != nil {
			log.Println(err)
			ServeError(w, r, fmt.Sprintf("Internal server error"), 500)
			return
		}
	}
	if err := rows.Err(); err != nil {
		log.Println(err)
		ServeError(w, r, fmt.Sprintf("Internal server error"), 500)
		return
	}

	generated_hash := t.generate_password_hash([]byte(pass), []byte(password_salt))

	if string(generated_hash) == password_hash {
		// Successful login
		ServeError(w, r, "Not implemented", 501)
	} else {
		// Invalid username / password combination
		ServeError(w, r, "Invalid username and/or password", 200)
	}
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
		log.Println(err)
		ServeError(w, r, fmt.Sprintf("Internal server error"), 500)
		return
	}
	if name_exists {
		ServeError(w, r, fmt.Sprintf("Username %s is already taken", user), 200)
		return
	}

	password_salt := t.generate_random_bytestring(64)
	password_hash := t.generate_password_hash([]byte(pass), password_salt)

	rows, err := t.db.Query(`INSERT INTO  degreesheep.user (
        username, password, password_salt, email, first_name,
        last_name, class_year ) VALUES ( ?, ?, ?, ?, ?, ?, ?)`, user, password_hash, password_salt, email, firstname, lastname, classyear)
	if err != nil {
		log.Println(err)
		ServeError(w, r, fmt.Sprintf("Internal server error"), 500)
		return
	}
	defer rows.Close()

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

// Generate a PBKDF password hash. Use 4096 iterations and a 64 byte key.
func (t *UserServlet) generate_password_hash(password, salt []byte) []byte {
	return pbkdf2.Key(password, salt, 4096, 64, sha256.New)
}
