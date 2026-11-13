package app

import (
	"encoding/json"
	"image"
	"io"
	"net/http"
	"strconv"
)

func (s *Server) appendPhotos(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	cat, err := s.Store.Get(r.Context(), id)
	if err != nil {
		problem(w, 404, "档案不存在")
		return
	}
	if s.Host.BaseURL == "" || s.Host.Token == "" {
		problem(w, 503, "图床尚未配置")
		return
	}
	select {
	case s.uploads <- struct{}{}:
		defer func() { <-s.uploads }()
	default:
		problem(w, 429, "图片处理繁忙，请稍后重试")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 52<<20)
	defer func() {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}()
	if r.ParseMultipartForm(1<<20) != nil {
		problem(w, 413, "相册过大或无法读取")
		return
	}
	files := r.MultipartForm.File["photos"]
	if len(files) == 0 {
		problem(w, 400, "请选择至少 1 张照片")
		return
	}
	var totalBytes int64
	for _, f := range files {
		totalBytes += f.Size
		if totalBytes > 50<<20 {
			problem(w, 413, "相册总大小不能超过 50 MB")
			return
		}
		if f.Size > 10<<20 {
			problem(w, 413, "单张照片不能超过 10 MB")
			return
		}
		src, e := f.Open()
		if e != nil {
			problem(w, 400, "无法读取图片")
			return
		}
		cfg, format, e := image.DecodeConfig(src)
		src.Close()
		if e != nil || !validChoice(format, "jpeg", "png", "webp") || cfg.Width <= 0 || cfg.Height <= 0 || int64(cfg.Width)*int64(cfg.Height) > 32000000 {
			problem(w, 400, "图片需为 JPEG、PNG、WebP，且不超过 3200 万像素")
			return
		}
	}
	photos := []Photo{}
	saved := false
	defer func() {
		if !saved {
			s.Store.Orphans(photos)
		}
	}()
	for _, f := range files {
		src, e := f.Open()
		if e != nil {
			problem(w, 400, "无法读取图片")
			return
		}
		data, e := io.ReadAll(io.LimitReader(src, (10<<20)+1))
		src.Close()
		if e != nil {
			problem(w, 400, "无法读取图片")
			return
		}
		p, e := s.Host.UploadPhoto(r.Context(), data, cat.AlbumKey, cat.Name)
		if p.ImageID > 0 {
			photos = append(photos, p)
		}
		if e != nil {
			problem(w, 502, e.Error())
			return
		}
	}
	tx, err := s.Store.db.BeginTx(r.Context(), nil)
	if err != nil {
		problem(w, 500, "相册保存失败")
		return
	}
	defer tx.Rollback()
	latest, err := scanCat(tx.QueryRowContext(r.Context(), "SELECT "+columns+" FROM cats WHERE id=?", id))
	if err != nil {
		problem(w, 409, "档案已变更，请刷新重试")
		return
	}
	latest.Photos = append(latest.Photos, photos...)
	b, _ := json.Marshal(latest.Photos)
	_, err = tx.ExecContext(r.Context(), "UPDATE cats SET photos=? WHERE id=?", string(b), id)
	if err == nil {
		err = auditTx(r.Context(), tx, "管理员", "cat.photos.append", strconv.FormatInt(id, 10), "{}")
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		problem(w, 500, "相册保存失败")
		return
	}
	saved = true
	writeJSON(w, 200, latest)
}
