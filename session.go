package main

import (
	"code.google.com/p/go-uuid/uuid"
	"database/sql"
	_ "github.com/go-sql-driver/mysql"
	"log"
	"math/rand"
	"sync"
	"time"
)

type SessionManager struct {
	db *sql.DB

	// Internal session cache
	sessions       map[string]*Session
	cache_capacity int
	cache_use      int
	cache_lock     sync.Mutex
}

type Session struct {
	user    *UserData
	expires time.Time
}

func NewSessionManager(server_config Config) *SessionManager {
	t := new(SessionManager)

	// Set up internal session cache
	t.sessions = make(map[string]*Session)
	t.cache_capacity = server_config.Cache.SessionCacheSize
	t.cache_use = 0
	t.cache_lock = sync.Mutex{}

	// Set up database connection
	db, err := sql.Open("mysql", server_config.GetSqlURI())
	if err != nil {
		log.Fatal("NewSessionManager", "Failed to open database:", err)
	}
	t.db = db
	return t
}

// Create a session, add it to the cache and plug it into the DB.
func (t *SessionManager) CreateSessionForUser(uid int) (string, error) {
	session_uuid := uuid.New()

	// Get the user's info
	user_data, err := FetchUserById(t.db, uid)
	if err != nil {
		return "", err
	}

	// Create the session object and put it in the local cache
	user_session := new(Session)
	user_session.user = user_data
	user_session.expires = time.Now().Add(48 * time.Hour)
	t.add_session_to_cache(session_uuid, user_session)

	// Store the token in the database
	_, err = t.db.Exec(`INSERT INTO  degreesheep.user_session (
		token, user_id, expire_time ) VALUES (?, ?, ?)`, session_uuid, uid, user_session.expires)
	if err != nil {
		// This isn't a fatal error since the session will be known by this API
		// server, but the session will be lost if the api server is restarted.
		// Can also lead to premature expiry in highly available API clusters.
		log.Println("CreateSessionForUser", err)
	}

	return session_uuid, nil
}

// Deletes a session from the database and local cache
func (t *SessionManager) DestroySession(session_uuid string) error {
	_, err := t.db.Exec("DELETE FROM  degreesheep.user_session WHERE token = ?", session_uuid)
	return err
}

// Fetch the session specified by a UUID. Returns whether the session exists,
// the session (if it exists) and an error.
func (t *SessionManager) GetSession(session_uuid string) (session_exists bool, session *Session, err error) {
	err = nil

	// If the session is still in the cache, we can safely return
	session, session_exists = t.sessions[session_uuid]
	if session_exists {
		return
	}

	// If it wasn't loaded into the cache, check if it's in the database.
	in_db, uid, expires, err := t.get_session_from_db(session_uuid)
	if err != nil {
		return false, nil, err
	}
	if in_db {
		// Load the session back into the cache and return it
		user_session := new(Session)
		user_session.user, err = FetchUserById(t.db, uid)
		if err != nil {
			return false, nil, err
		}
		user_session.expires = expires
		t.add_session_to_cache(session_uuid, user_session)
		return true, user_session, nil
	}

	// If it isn't in cache or DB, return false.
	return false, nil, nil
}

// Check if a session exists in the database and is still valid.
// Returns three values - whether the token exists & is valid, the user id and
// an error.
func (t *SessionManager) get_session_from_db(session_uuid string) (exists bool, user_id int, expire_time time.Time, err error) {

	rows, err := t.db.Query("SELECT user_id, expire_time FROM degreesheep.user_session WHERE token = ? AND expire_time > CURRENT_TIMESTAMP()", session_uuid)
	if err != nil {
		return false, 0, time.Now(), err
	}
	defer rows.Close()

	num_rows := 0
	for rows.Next() {
		num_rows++
		if err := rows.Scan(&user_id, &expire_time); err != nil {
			return false, 0, time.Now(), err
		}
	}
	if err := rows.Err(); err != nil {
		return false, 0, time.Now(), err
	}
	// If we got no rows, the session is invalid / expired.
	if num_rows == 0 {
		log.Println("get_session_from_db", "No db rows found for session", session_uuid)
		return false, 0, time.Now(), nil
	}

	// Otherwise, we got a valid token
	return true, user_id, expire_time, nil
}

// Adds a session to the cache. Will also prune the cache if it is full.
func (t *SessionManager) add_session_to_cache(session_uuid string, session *Session) {
	t.cache_lock.Lock()

	// If the capacity of the cache is reached, we need to prune
	if t.cache_use == t.cache_capacity {
		now := time.Now()
		for session_id, session := range t.sessions {
			// If the sessions's expiry time is before now, remove it
			if session.expires.Before(now) {
				delete(t.sessions, session_id)
				t.cache_use -= 1
			}
		}

		// Check if we managed to free up any capacity. If not, we need to prune
		// active sessions from the cache
		if t.cache_use == t.cache_capacity {
			// For the sake of speed, don't bother to sort and remove the oldest
			// sessions. The cost of reloading from the DB is low and infrequent
			// so just delete a quarter of sessions.
			random := rand.New(rand.NewSource(time.Now().UnixNano()))
			for session_id := range t.sessions {
				if random.Int()%4 == 0 {
					delete(t.sessions, session_id)
					t.cache_use -= 1
				}
			}
		}
	}

	// Put the new session into the cache
	t.sessions[session_uuid] = session

	t.cache_lock.Unlock()
}
