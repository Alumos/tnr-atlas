package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"time"
)

func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 32<<10)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if d.Decode(v) != nil {
		problem(w, 400, "请求内容无效")
		return false
	}
	return true
}
func (s *Server) adminOnly(fn http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a, ok := s.actor(r, "tnr_admin")
		if !ok || a.Role != "admin" {
			problem(w, 401, "请先登录管理员后台")
			return
		}
		fn(w, r)
	}
}
func (s *Server) adminRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/admin/session", s.adminLogin)
	mux.HandleFunc("GET /api/admin/session", func(w http.ResponseWriter, r *http.Request) {
		a, ok := s.actor(r, "tnr_admin")
		writeJSON(w, 200, map[string]bool{"authenticated": ok && a.Role == "admin"})
	})
	mux.HandleFunc("DELETE /api/admin/session", s.adminOnly(func(w http.ResponseWriter, r *http.Request) {
		if c, e := r.Cookie("tnr_admin"); e == nil {
			_, _ = s.Store.db.ExecContext(r.Context(), "DELETE FROM sessions WHERE token_hash=?", digest(c.Value))
		}
		http.SetCookie(w, &http.Cookie{Name: "tnr_admin", Value: "", Path: "/api", HttpOnly: true, Secure: s.SecureCookie, SameSite: http.SameSiteStrictMode, MaxAge: -1})
		writeJSON(w, 200, map[string]bool{"ok": true})
	}))
	mux.HandleFunc("GET /api/admin/settings", s.adminOnly(func(w http.ResponseWriter, r *http.Request) {
		enabled, err := s.Store.setting(r.Context(), "shared_enabled")
		if err != nil {
			problem(w, 500, "设置加载失败")
			return
		}
		writeJSON(w, 200, map[string]any{"sharedEnabled": enabled == "true", "imageRetention": "keep"})
	}))
	mux.HandleFunc("PUT /api/admin/settings", s.adminOnly(s.updateSettings))
	mux.HandleFunc("GET /api/admin/volunteers", s.adminOnly(s.volunteers))
	mux.HandleFunc("POST /api/admin/volunteers", s.adminOnly(s.saveVolunteer))
	mux.HandleFunc("PUT /api/admin/volunteers/{id}", s.adminOnly(s.saveVolunteer))
	mux.HandleFunc("DELETE /api/admin/volunteers/{id}", s.adminOnly(s.deleteVolunteer))
	mux.HandleFunc("PUT /api/admin/cats/{id}", s.adminOnly(s.editCat))
	mux.HandleFunc("POST /api/admin/cats/{id}/photos", s.adminOnly(s.appendPhotos))
	mux.HandleFunc("DELETE /api/admin/cats/{id}", s.adminOnly(s.deleteCat))
	mux.HandleFunc("GET /api/admin/audit", s.adminOnly(s.auditList))
}
func txResult(w http.ResponseWriter, tx *sql.Tx, err error) {
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		problem(w, 500, "操作未保存，请重试")
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}
func (s *Server) updateSettings(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Code          string `json:"code"`
		SharedEnabled bool   `json:"sharedEnabled"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	hash := ""
	var err error
	if body.Code != "" {
		hash, err = hashCode(body.Code)
		if err != nil {
			problem(w, 400, err.Error())
			return
		}
	}
	tx, err := s.Store.db.BeginTx(r.Context(), nil)
	if err != nil {
		problem(w, 500, "设置暂时不可用")
		return
	}
	defer tx.Rollback()
	if hash != "" {
		_, err = tx.ExecContext(r.Context(), "UPDATE settings SET value=? WHERE key='access_hash'", hash)
	}
	if err == nil {
		_, err = tx.ExecContext(r.Context(), "UPDATE settings SET value=? WHERE key='shared_enabled'", strconv.FormatBool(body.SharedEnabled))
	}
	if err == nil {
		_, err = tx.ExecContext(r.Context(), "DELETE FROM sessions WHERE role='shared'")
	}
	if err == nil {
		details, _ := json.Marshal(map[string]bool{"codeChanged": hash != "", "sharedEnabled": body.SharedEnabled})
		err = auditTx(r.Context(), tx, "管理员", "settings.update", "access", string(details))
	}
	txResult(w, tx, err)
}
func (s *Server) volunteers(w http.ResponseWriter, r *http.Request) {
	rows, err := s.Store.db.QueryContext(r.Context(), "SELECT id,username,name,active,created_at FROM volunteers ORDER BY id DESC")
	if err != nil {
		problem(w, 500, "志愿者加载失败")
		return
	}
	defer rows.Close()
	items := []Volunteer{}
	for rows.Next() {
		var v Volunteer
		if err = rows.Scan(&v.ID, &v.Username, &v.Name, &v.Active, &v.CreatedAt); err != nil {
			break
		}
		items = append(items, v)
	}
	if err != nil || rows.Err() != nil {
		problem(w, 500, "志愿者加载失败")
		return
	}
	writeJSON(w, 200, items)
}

var usernamePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{3,32}$`)

func (s *Server) saveVolunteer(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Username string `json:"username"`
		Name     string `json:"name"`
		Code     string `json:"code"`
		Active   bool   `json:"active"`
	}
	if !decodeJSON(w, r, &b) {
		return
	}
	if !usernamePattern.MatchString(b.Username) || len([]rune(b.Name)) < 1 || len([]rune(b.Name)) > 40 {
		problem(w, 400, "账号需为 3–32 位字母、数字、下划线；昵称需为 1–40 字")
		return
	}
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if r.Method == "PUT" && id <= 0 {
		problem(w, 400, "志愿者编号无效")
		return
	}
	var hash string
	var err error
	if b.Code != "" || id == 0 {
		hash, err = hashCode(b.Code)
		if err != nil {
			problem(w, 400, err.Error())
			return
		}
	}
	tx, err := s.Store.db.BeginTx(r.Context(), nil)
	if err != nil {
		problem(w, 500, "暂时无法保存")
		return
	}
	defer tx.Rollback()
	action := "volunteer.create"
	if id == 0 {
		var res sql.Result
		res, err = tx.ExecContext(r.Context(), "INSERT INTO volunteers(username,name,code_hash,active,created_at) VALUES(?,?,?,?,?)", b.Username, b.Name, hash, b.Active, time.Now().UTC().Format(time.RFC3339))
		if err == nil {
			id, err = res.LastInsertId()
		}
	} else {
		action = "volunteer.update"
		if hash == "" {
			err = tx.QueryRowContext(r.Context(), "SELECT code_hash FROM volunteers WHERE id=?", id).Scan(&hash)
		}
		if err == nil {
			_, err = tx.ExecContext(r.Context(), "UPDATE volunteers SET username=?,name=?,code_hash=?,active=? WHERE id=?", b.Username, b.Name, hash, b.Active, id)
		}
		if err == nil {
			_, err = tx.ExecContext(r.Context(), "DELETE FROM sessions WHERE volunteer_id=?", id)
		}
	}
	if err != nil {
		problem(w, 400, "保存失败：账号可能已存在，或该志愿者已被删除")
		return
	}
	if err == nil {
		details, _ := json.Marshal(map[string]any{"username": b.Username, "name": b.Name, "active": b.Active, "codeChanged": b.Code != ""})
		err = auditTx(r.Context(), tx, "管理员", action, fmt.Sprint(id), string(details))
	}
	txResult(w, tx, err)
}
func (s *Server) deleteVolunteer(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	tx, err := s.Store.db.BeginTx(r.Context(), nil)
	if err != nil {
		problem(w, 500, "暂时无法删除")
		return
	}
	defer tx.Rollback()
	var username string
	err = tx.QueryRowContext(r.Context(), "SELECT username FROM volunteers WHERE id=?", id).Scan(&username)
	if errors.Is(err, sql.ErrNoRows) {
		problem(w, 404, "志愿者不存在")
		return
	}
	if err == nil {
		_, err = tx.ExecContext(r.Context(), "DELETE FROM volunteers WHERE id=?", id)
	}
	if err == nil {
		_, err = tx.ExecContext(r.Context(), "DELETE FROM sessions WHERE volunteer_id=?", id)
	}
	if err == nil {
		details, _ := json.Marshal(map[string]string{"username": username})
		err = auditTx(r.Context(), tx, "管理员", "volunteer.delete", fmt.Sprint(id), string(details))
	}
	txResult(w, tx, err)
}
func (s *Server) editCat(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	var c Cat
	if !decodeJSON(w, r, &c) {
		return
	}
	if err := c.Validate(); err != nil {
		problem(w, 400, err.Error())
		return
	}
	if len(c.Photos) < 1 || len(c.Photos) > 6 {
		problem(w, 400, "相册需保留 1–6 张照片")
		return
	}
	tx, err := s.Store.db.BeginTx(r.Context(), nil)
	if err != nil {
		problem(w, 500, "暂时无法保存")
		return
	}
	defer tx.Rollback()
	old, err := scanCat(tx.QueryRowContext(r.Context(), "SELECT "+columns+" FROM cats WHERE id=?", id))
	if errors.Is(err, sql.ErrNoRows) {
		problem(w, 404, "猫咪档案不存在")
		return
	}
	if err != nil {
		problem(w, 500, "暂时无法加载")
		return
	}
	retained := []Photo{}
	seen := map[int64]bool{}
	for _, p := range c.Photos {
		found := false
		for _, existing := range old.Photos {
			if p.ImageID == existing.ImageID && !seen[p.ImageID] {
				retained = append(retained, existing)
				seen[p.ImageID] = true
				found = true
				break
			}
		}
		if !found {
			problem(w, 400, "相册内容无效，请刷新后重试")
			return
		}
	}
	if err == nil {
		b, _ := json.Marshal(retained)
		_, err = tx.ExecContext(r.Context(), "UPDATE cats SET name=?,sex=?,color=?,neutered=?,description=?,uploader=?,photos=? WHERE id=?", c.Name, c.Sex, c.Color, c.Neutered, c.Description, c.Uploader, string(b), id)
	}
	if err == nil {
		before, _ := json.Marshal(old)
		after := c
		after.ID = id
		after.AlbumKey = old.AlbumKey
		after.Photos = retained
		after.CreatedAt = old.CreatedAt
		details, _ := json.Marshal(map[string]any{"before": json.RawMessage(before), "after": after})
		err = auditTx(r.Context(), tx, "管理员", "cat.update", fmt.Sprint(id), string(details))
	}
	txResult(w, tx, err)
}
func (s *Server) deleteCat(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	tx, err := s.Store.db.BeginTx(r.Context(), nil)
	if err != nil {
		problem(w, 500, "暂时无法删除")
		return
	}
	defer tx.Rollback()
	c, err := scanCat(tx.QueryRowContext(r.Context(), "SELECT "+columns+" FROM cats WHERE id=?", id))
	if errors.Is(err, sql.ErrNoRows) {
		problem(w, 404, "猫咪档案不存在")
		return
	}
	if err == nil {
		_, err = tx.ExecContext(r.Context(), "DELETE FROM cats WHERE id=?", id)
	}
	if err == nil {
		details, _ := json.Marshal(c)
		err = auditTx(r.Context(), tx, "管理员", "cat.delete", fmt.Sprint(id), string(details))
	}
	txResult(w, tx, err)
}
func (s *Server) auditList(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	if page > 100000 {
		problem(w, 400, "页码超出范围")
		return
	}
	var total int
	if s.Store.db.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM audit").Scan(&total) != nil {
		problem(w, 500, "审计加载失败")
		return
	}
	rows, err := s.Store.db.QueryContext(r.Context(), "SELECT id,actor,action,target,details,created_at FROM audit ORDER BY id DESC LIMIT 50 OFFSET ?", (page-1)*50)
	if err != nil {
		problem(w, 500, "审计加载失败")
		return
	}
	defer rows.Close()
	items := []Audit{}
	for rows.Next() {
		var a Audit
		if err = rows.Scan(&a.ID, &a.Actor, &a.Action, &a.Target, &a.Details, &a.CreatedAt); err != nil {
			break
		}
		items = append(items, a)
	}
	if err != nil || rows.Err() != nil {
		problem(w, 500, "审计加载失败")
		return
	}
	writeJSON(w, 200, map[string]any{"items": items, "total": total, "page": page})
}

// Record uploaded files when a request fails; official oneimg retains the images.
func (s *Store) Orphans(photos []Photo) {
	if len(photos) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		slog.Error("could not record retained album")
		return
	}
	defer tx.Rollback()
	details, _ := json.Marshal(map[string]any{"photos": photos, "reason": "upload_not_committed", "imageRetention": "keep"})
	if err = auditTx(ctx, tx, "系统", "album.retained", "uncommitted", string(details)); err == nil {
		err = tx.Commit()
	}
	if err != nil {
		slog.Error("could not record retained album")
	}
}
