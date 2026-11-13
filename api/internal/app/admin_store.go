package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"golang.org/x/crypto/bcrypt"
	"time"
)

func digest(v string) string { s := sha256.Sum256([]byte(v)); return hex.EncodeToString(s[:]) }
func hashCode(v string) (string, error) {
	if len(v) < 16 || len(v) > 72 {
		return "", fmt.Errorf("访问码需为 16–72 字节")
	}
	b, e := bcrypt.GenerateFromPassword([]byte(v), bcrypt.DefaultCost)
	return string(b), e
}
func (s *Store) InitAuth(code string) error {
	_, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS settings(key TEXT PRIMARY KEY,value TEXT NOT NULL);
 CREATE TABLE IF NOT EXISTS volunteers(id INTEGER PRIMARY KEY AUTOINCREMENT,username TEXT NOT NULL UNIQUE,name TEXT NOT NULL,code_hash TEXT NOT NULL,active INTEGER NOT NULL DEFAULT 1,created_at TEXT NOT NULL);
 CREATE TABLE IF NOT EXISTS sessions(token_hash TEXT PRIMARY KEY,role TEXT NOT NULL,name TEXT NOT NULL,volunteer_id INTEGER NOT NULL DEFAULT 0,credential TEXT NOT NULL,expires INTEGER NOT NULL);
 CREATE TABLE IF NOT EXISTS audit(id INTEGER PRIMARY KEY AUTOINCREMENT,actor TEXT NOT NULL,action TEXT NOT NULL,target TEXT NOT NULL,details TEXT NOT NULL,created_at TEXT NOT NULL);
 CREATE INDEX IF NOT EXISTS session_expiry ON sessions(expires);`)
	if err != nil {
		return err
	}
	var count int
	if err = s.db.QueryRow("SELECT COUNT(*) FROM settings WHERE key='access_hash'").Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		h, err := hashCode(code)
		if err != nil {
			return err
		}
		_, err = s.db.Exec("INSERT INTO settings(key,value) VALUES('access_hash',?),('shared_enabled','true')", h)
		return err
	}
	return nil
}

type Actor struct {
	Role, Name, Credential string
	VolunteerID            int64
}

func (a Actor) AuditName() string {
	if a.Role == "volunteer" {
		return fmt.Sprintf("%s [志愿者#%d]", a.Name, a.VolunteerID)
	}
	return a.Name
}

func (s *Store) setting(ctx context.Context, key string) (string, error) {
	var value string
	err := s.db.QueryRowContext(ctx, "SELECT value FROM settings WHERE key=?", key).Scan(&value)
	return value, err
}
func (s *Store) newSession(ctx context.Context, a Actor) (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	token := hex.EncodeToString(buf)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "DELETE FROM sessions WHERE expires<?", time.Now().Unix()); err != nil {
		return "", err
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO sessions VALUES(?,?,?,?,?,?)", digest(token), a.Role, a.Name, a.VolunteerID, a.Credential, time.Now().Add(12*time.Hour).Unix()); err != nil {
		return "", err
	}
	if err = auditTx(ctx, tx, a.AuditName(), "session.login", a.Role, "{}"); err != nil {
		return "", err
	}
	return token, tx.Commit()
}
func auditTx(ctx context.Context, tx *sql.Tx, actor, action, target, details string) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO audit(actor,action,target,details,created_at) VALUES(?,?,?,?,?)", actor, action, target, details, time.Now().UTC().Format(time.RFC3339))
	return err
}
func (s *Store) CreateAudited(ctx context.Context, c *Cat, actor string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	photos, err := json.Marshal(c.Photos)
	if err != nil {
		return err
	}
	c.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	res, err := tx.ExecContext(ctx, "INSERT INTO cats(name,sex,color,neutered,description,uploader,photos,created_at,album_key) VALUES(?,?,?,?,?,?,?,?,?)", c.Name, c.Sex, c.Color, c.Neutered, c.Description, c.Uploader, string(photos), c.CreatedAt, c.AlbumKey)
	if err != nil {
		return err
	}
	c.ID, err = res.LastInsertId()
	if err != nil {
		return err
	}
	if err = auditTx(ctx, tx, actor, "cat.create", fmt.Sprint(c.ID), "{}"); err != nil {
		return err
	}
	return tx.Commit()
}

type Volunteer struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	Name      string `json:"name"`
	Active    bool   `json:"active"`
	CreatedAt string `json:"createdAt"`
}
type Audit struct {
	ID        int64  `json:"id"`
	Actor     string `json:"actor"`
	Action    string `json:"action"`
	Target    string `json:"target"`
	Details   string `json:"details"`
	CreatedAt string `json:"createdAt"`
}
