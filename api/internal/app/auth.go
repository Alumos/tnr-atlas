package app

import (
	"crypto/subtle"
	"encoding/json"
	"golang.org/x/crypto/bcrypt"
	"net/http"
	"time"
)

func (s *Server) actor(r *http.Request, cookieName string) (Actor, bool) {
	var a Actor
	c, err := r.Cookie(cookieName)
	if err != nil || len(c.Value) != 64 {
		return a, false
	}
	err = s.Store.db.QueryRowContext(r.Context(), "SELECT role,name,volunteer_id,credential FROM sessions WHERE token_hash=? AND expires>?", digest(c.Value), time.Now().Unix()).Scan(&a.Role, &a.Name, &a.VolunteerID, &a.Credential)
	if err != nil {
		return a, false
	}
	switch a.Role {
	case "admin":
		return a, s.AdminPassword != "" && a.Credential == digest(s.AdminPassword)
	case "shared":
		h, err := s.Store.setting(r.Context(), "access_hash")
		enabled, _ := s.Store.setting(r.Context(), "shared_enabled")
		return a, err == nil && enabled == "true" && a.Credential == h
	case "volunteer":
		var hash, name string
		var active bool
		err := s.Store.db.QueryRowContext(r.Context(), "SELECT code_hash,active,name FROM volunteers WHERE id=?", a.VolunteerID).Scan(&hash, &active, &name)
		a.Name = name
		return a, err == nil && active && a.Credential == hash
	}
	return a, false
}
func (s *Server) authorized(r *http.Request) bool { _, ok := s.actor(r, "tnr_session"); return ok }
func (s *Server) limited() bool {
	s.loginMu.Lock()
	defer s.loginMu.Unlock()
	if time.Since(s.loginWindow) > time.Minute {
		s.loginWindow = time.Now()
		s.loginCount = 0
	}
	s.loginCount++
	return s.loginCount > 20
}
func (s *Server) issueSession(w http.ResponseWriter, r *http.Request, a Actor, cookie string) {
	token, err := s.Store.newSession(r.Context(), a)
	if err != nil {
		problem(w, 500, "登录暂时不可用")
		return
	}
	http.SetCookie(w, &http.Cookie{Name: cookie, Value: token, Path: "/api", HttpOnly: true, Secure: s.SecureCookie, SameSite: http.SameSiteStrictMode, MaxAge: 43200})
	writeJSON(w, 200, map[string]any{"authenticated": true, "name": a.Name})
}
func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	if s.limited() {
		w.Header().Set("Retry-After", "60")
		problem(w, 429, "尝试过多，请一分钟后重试")
		return
	}
	var body struct {
		Username string `json:"username"`
		Code     string `json:"code"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	if json.NewDecoder(r.Body).Decode(&body) != nil || len(body.Code) > 72 {
		problem(w, 400, "请输入有效的访问码")
		return
	}
	a := Actor{Role: "shared", Name: "公共入口"}
	var err error
	if body.Username != "" {
		a.Role = "volunteer"
		var active bool
		err = s.Store.db.QueryRowContext(r.Context(), "SELECT id,name,code_hash,active FROM volunteers WHERE username=?", body.Username).Scan(&a.VolunteerID, &a.Name, &a.Credential, &active)
		if !active {
			err = http.ErrNoCookie
		}
	} else {
		a.Credential, err = s.Store.setting(r.Context(), "access_hash")
		enabled, _ := s.Store.setting(r.Context(), "shared_enabled")
		if enabled != "true" {
			err = http.ErrNoCookie
		}
	}
	hash := a.Credential
	if hash == "" {
		hash = "$2a$10$wMRzY1dmN/u9N2ZJqliZmeSBYNsW7ADe12qhOCEo09.1zms/IMzrS"
	}
	valid := bcrypt.CompareHashAndPassword([]byte(hash), []byte(body.Code)) == nil
	if err != nil || !valid {
		problem(w, 401, "账号或访问码不正确，或入口已停用")
		return
	}
	s.issueSession(w, r, a, "tnr_session")
}
func (s *Server) adminLogin(w http.ResponseWriter, r *http.Request) {
	if s.limited() {
		problem(w, 429, "尝试过多，请一分钟后重试")
		return
	}
	var body struct {
		Password string `json:"password"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	if json.NewDecoder(r.Body).Decode(&body) != nil || s.AdminPassword == "" || subtle.ConstantTimeCompare([]byte(digest(body.Password)), []byte(digest(s.AdminPassword))) != 1 {
		problem(w, 401, "管理员密码不正确")
		return
	}
	s.issueSession(w, r, Actor{Role: "admin", Name: "管理员", Credential: digest(s.AdminPassword)}, "tnr_admin")
}
