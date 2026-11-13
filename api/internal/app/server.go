package app

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Server struct {
	Store                 *Store
	Host                  *ImageHost
	AdminPassword, Origin string
	SecureCookie          bool
	uploads               chan struct{}
	loginMu               sync.Mutex
	loginWindow           time.Time
	loginCount            int
}

func NewServer(store *Store, host *ImageHost, code, origin string) *Server {
	_ = code
	return &Server{Store: store, Host: host, Origin: origin, SecureCookie: strings.HasPrefix(origin, "https://"), uploads: make(chan struct{}, 2)}
}
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func problem(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	s.adminRoutes(mux)
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		if err := s.Store.db.PingContext(r.Context()); err != nil {
			problem(w, 503, "数据库暂时不可用")
			return
		}
		writeJSON(w, 200, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /api/session", func(w http.ResponseWriter, r *http.Request) {
		a, ok := s.actor(r, "tnr_session")
		writeJSON(w, 200, map[string]any{"authenticated": ok, "uploadsReady": s.Host.BaseURL != "" && s.Host.Token != "", "role": a.Role, "name": a.Name})
	})
	mux.HandleFunc("POST /api/session", s.login)
	mux.HandleFunc("DELETE /api/session", func(w http.ResponseWriter, r *http.Request) {
		if c, e := r.Cookie("tnr_session"); e == nil {
			_, _ = s.Store.db.ExecContext(r.Context(), "DELETE FROM sessions WHERE token_hash=?", digest(c.Value))
		}
		http.SetCookie(w, &http.Cookie{Name: "tnr_session", Value: "", Path: "/api", MaxAge: -1, HttpOnly: true, Secure: s.SecureCookie, SameSite: http.SameSiteStrictMode})
		writeJSON(w, 200, map[string]bool{"ok": true})
	})
	mux.HandleFunc("GET /api/stats", func(w http.ResponseWriter, r *http.Request) {
		v, err := s.Store.Stats(r.Context())
		if err != nil {
			problem(w, 500, "统计暂时不可用")
			return
		}
		writeJSON(w, 200, v)
	})
	mux.HandleFunc("GET /api/cats", s.list)
	mux.HandleFunc("GET /api/cats/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			problem(w, 400, "无效的档案编号")
			return
		}
		c, err := s.Store.Get(r.Context(), id)
		if errors.Is(err, sql.ErrNoRows) {
			problem(w, 404, "档案不存在")
			return
		}
		if err != nil {
			problem(w, 500, "档案暂时不可用")
			return
		}
		writeJSON(w, 200, c)
	})
	mux.HandleFunc("POST /api/cats", s.create)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		if r.Method != "GET" && r.Method != "HEAD" {
			origin := r.Header.Get("Origin")
			expected := s.Origin
			if expected == "" {
				scheme := "http"
				if r.TLS != nil {
					scheme = "https"
				}
				expected = scheme + "://" + r.Host
			}
			if origin != "" && origin != expected {
				problem(w, 403, "请求来源无效")
				return
			}
		}
		defer func() {
			if recover() != nil {
				slog.Error("request panic recovered")
				problem(w, 500, "服务暂时不可用")
			}
		}()
		mux.ServeHTTP(w, r)
	})
}
func (s *Server) list(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	if page < 1 {
		page = 1
	}
	if page > 100000 {
		problem(w, 400, "页码超出范围")
		return
	}
	search := q.Get("q")
	if len([]rune(search)) > 100 {
		problem(w, 400, "搜索内容最多 100 字")
		return
	}
	cats, total, err := s.Store.List(r.Context(), search, q.Get("sex"), q.Get("color"), q.Get("status"), page)
	if err != nil {
		problem(w, 500, "档案加载失败")
		return
	}
	writeJSON(w, 200, map[string]any{"items": cats, "total": total, "page": page, "pageSize": 24})
}
func (s *Server) create(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.actor(r, "tnr_session")
	if !ok {
		problem(w, 401, "请先输入志愿者访问码")
		return
	}
	if s.Host.BaseURL == "" || s.Host.Token == "" {
		problem(w, 503, "图床尚未配置，请联系管理员")
		return
	}
	select {
	case s.uploads <- struct{}{}:
		defer func() { <-s.uploads }()
	default:
		w.Header().Set("Retry-After", "10")
		problem(w, 429, "正在处理其他相册，请稍后重试")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 52<<20)
	defer func() {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}()
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		var large *http.MaxBytesError
		if errors.As(err, &large) {
			problem(w, 413, "相册总大小不能超过 50 MB")
		} else {
			problem(w, 400, "无法读取相册，请重新选择图片")
		}
		return
	}
	c := Cat{Name: r.FormValue("name"), Sex: r.FormValue("sex"), Color: r.FormValue("color"), Neutered: r.FormValue("neutered"), NeuteredAt: r.FormValue("neuteredAt"), Description: r.FormValue("description"), Uploader: r.FormValue("uploader")}
	if actor.Role == "volunteer" {
		c.Uploader = actor.Name
	}
	if err := c.Validate(); err != nil {
		problem(w, 400, err.Error())
		return
	}
	files := r.MultipartForm.File["photos"]
	if len(files) < 1 {
		problem(w, 400, "相册至少需要 1 张照片")
		return
	}
	// Validate every file header before making any remote write.
	var totalBytes int64
	for _, f := range files {
		totalBytes += f.Size
		if totalBytes > 50<<20 {
			problem(w, 413, "相册总大小不能超过 50 MB")
			return
		}
		if f.Size > 10<<20 {
			problem(w, 413, "每张照片不能超过 10 MB")
			return
		}
		src, err := f.Open()
		if err != nil {
			problem(w, 400, "无法读取照片")
			return
		}
		data, err := io.ReadAll(io.LimitReader(src, (10<<20)+1))
		src.Close()
		if err != nil {
			problem(w, 400, "无法读取照片")
			return
		}
		cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil || !validChoice(format, "jpeg", "png", "webp") || cfg.Width <= 0 || cfg.Height <= 0 || int64(cfg.Width)*int64(cfg.Height) > 32000000 {
			problem(w, 400, "照片需要为 JPEG、PNG 或 WebP，且不超过 3200 万像素")
			return
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), 4*time.Minute)
	defer cancel()
	c.AlbumKey = randomKey()
	saved := false
	defer func() {
		if !saved && len(c.Photos) > 0 {
			s.Store.Orphans(c.Photos)
		}
	}()
	for i, f := range files {
		src, err := f.Open()
		if err != nil {
			problem(w, 400, "无法读取照片")
			return
		}
		data, err := io.ReadAll(src)
		src.Close()
		if err != nil {
			problem(w, 400, "无法读取照片")
			return
		}
		p, err := s.Host.UploadPhoto(ctx, data, c.AlbumKey, c.Name)
		if p.ImageID > 0 {
			c.Photos = append(c.Photos, p)
		}
		if err != nil {
			slog.Warn("album upload failed", "photo", i+1)
			problem(w, 502, fmt.Sprintf("第 %d 张照片上传失败：%s。档案尚未保存，请重试。", i+1, err.Error()))
			return
		}
	}
	if err := s.Store.CreateAudited(ctx, &c, actor.AuditName()); err != nil {
		problem(w, 500, "图片已上传，但档案保存失败，请联系管理员后重试")
		return
	}
	saved = true
	writeJSON(w, 201, c)
}
func ValidateConfig(code, origin, host string) error {
	if len(code) < 16 {
		return fmt.Errorf("TNR_ACCESS_CODE must contain at least 16 characters")
	}
	for _, v := range []string{origin, host} {
		if v != "" {
			u, err := url.Parse(v)
			if err != nil || u.Host == "" || u.User != nil || !validChoice(u.Scheme, "http", "https") {
				return fmt.Errorf("invalid service URL")
			}
		}
	}
	return nil
}
