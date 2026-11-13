package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	_ "modernc.org/sqlite"
	"strings"
	"time"
)

var Colors = []string{"橘猫", "狸花", "三花", "奶牛", "黑猫", "白猫", "玳瑁", "灰猫", "橘白", "无法定义"}

type Photo struct {
	ImageID   int64  `json:"imageId"`
	URL       string `json:"url"`
	Thumbnail string `json:"thumbnail"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
}
type Cat struct {
	AlbumKey    string  `json:"albumKey"`
	ID          int64   `json:"id"`
	Name        string  `json:"name"`
	Sex         string  `json:"sex"`
	Color       string  `json:"color"`
	Neutered    string  `json:"neutered"`
	Description string  `json:"description"`
	Uploader    string  `json:"uploader"`
	Photos      []Photo `json:"photos"`
	CreatedAt   string  `json:"createdAt"`
}
type Store struct{ db *sql.DB }

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	_, err = db.Exec(`PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000; PRAGMA foreign_keys=ON;
 CREATE TABLE IF NOT EXISTS cats(id INTEGER PRIMARY KEY AUTOINCREMENT,name TEXT NOT NULL,sex TEXT NOT NULL,color TEXT NOT NULL,neutered TEXT NOT NULL,description TEXT NOT NULL,uploader TEXT NOT NULL,photos TEXT NOT NULL,created_at TEXT NOT NULL,album_key TEXT NOT NULL DEFAULT '');
 CREATE INDEX IF NOT EXISTS cats_status ON cats(neutered,id DESC);`)
	if err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db}, nil
}
func (s *Store) Close() error { return s.db.Close() }
func validChoice(s string, choices ...string) bool {
	for _, v := range choices {
		if s == v {
			return true
		}
	}
	return false
}
func (c *Cat) Validate() error {
	c.Name = strings.TrimSpace(c.Name)
	c.Uploader = strings.TrimSpace(c.Uploader)
	c.Description = strings.TrimSpace(c.Description)
	if len([]rune(c.Name)) < 1 || len([]rune(c.Name)) > 40 {
		return fmt.Errorf("猫咪名字需为 1–40 个字")
	}
	if len([]rune(c.Uploader)) < 1 || len([]rune(c.Uploader)) > 40 {
		return fmt.Errorf("上传人需为 1–40 个字")
	}
	if !validChoice(c.Sex, "公", "母", "未知") || !validChoice(c.Color, Colors...) || !validChoice(c.Neutered, "已绝育", "未绝育", "未知") {
		return fmt.Errorf("请选择有效的性别、毛色与绝育状态")
	}
	if len([]rune(c.Description)) > 2000 {
		return fmt.Errorf("补充描述最多 2000 个字")
	}
	return nil
}
func (s *Store) Create(ctx context.Context, c *Cat) error {
	photos, err := json.Marshal(c.Photos)
	if err != nil {
		return err
	}
	c.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.ExecContext(ctx, "INSERT INTO cats(name,sex,color,neutered,description,uploader,photos,created_at,album_key) VALUES(?,?,?,?,?,?,?,?,?)", c.Name, c.Sex, c.Color, c.Neutered, c.Description, c.Uploader, string(photos), c.CreatedAt, c.AlbumKey)
	if err != nil {
		return err
	}
	c.ID, err = res.LastInsertId()
	return err
}
func scanCat(row interface{ Scan(...any) error }) (Cat, error) {
	var c Cat
	var photos string
	err := row.Scan(&c.ID, &c.Name, &c.Sex, &c.Color, &c.Neutered, &c.Description, &c.Uploader, &photos, &c.CreatedAt, &c.AlbumKey)
	if err == nil {
		err = json.Unmarshal([]byte(photos), &c.Photos)
	}
	return c, err
}

const columns = "id,name,sex,color,neutered,description,uploader,photos,created_at,album_key"

func (s *Store) Get(ctx context.Context, id int64) (Cat, error) {
	return scanCat(s.db.QueryRowContext(ctx, "SELECT "+columns+" FROM cats WHERE id=?", id))
}

type Stats struct {
	Total        int `json:"total"`
	Neutered     int `json:"neutered"`
	Pending      int `json:"pending"`
	Unknown      int `json:"unknown"`
	Contributors int `json:"contributors"`
}

func (s *Store) Stats(ctx context.Context) (Stats, error) {
	var v Stats
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*),COALESCE(SUM(neutered='已绝育'),0),COALESCE(SUM(neutered='未绝育'),0),COALESCE(SUM(neutered='未知'),0),COUNT(DISTINCT uploader) FROM cats`).Scan(&v.Total, &v.Neutered, &v.Pending, &v.Unknown, &v.Contributors)
	return v, err
}
func (s *Store) List(ctx context.Context, q, sex, color, status string, page int) ([]Cat, int, error) {
	where := " WHERE 1=1"
	args := []any{}
	if q != "" {
		where += ` AND (name LIKE ? ESCAPE '\' OR description LIKE ? ESCAPE '\' OR uploader LIKE ? ESCAPE '\')`
		q = strings.NewReplacer("\\", "\\\\", "%", "\\%", "_", "\\_").Replace(q)
		for range 3 {
			args = append(args, "%"+q+"%")
		}
	}
	for _, f := range []struct{ k, v string }{{"sex", sex}, {"color", color}, {"neutered", status}} {
		if f.v != "" {
			where += " AND " + f.k + "=?"
			args = append(args, f.v)
		}
	}
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM cats"+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, 24, (page-1)*24)
	rows, err := s.db.QueryContext(ctx, "SELECT "+columns+" FROM cats"+where+" ORDER BY id DESC LIMIT ? OFFSET ?", args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	cats := []Cat{}
	for rows.Next() {
		c, err := scanCat(rows)
		if err != nil {
			return nil, 0, err
		}
		cats = append(cats, c)
	}
	return cats, total, rows.Err()
}
