package app

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	_ "golang.org/x/image/webp"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strings"
	"time"
	"unicode"
)

type ImageHost struct {
	BaseURL, Token string
	FolderID       int64
	Client         *http.Client
}

func NewImageHost(base, token string) *ImageHost {
	return &ImageHost{BaseURL: strings.TrimRight(base, "/"), Token: token, Client: &http.Client{Timeout: 90 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}}
}
func imageFormat(data []byte) (string, error) {
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("仅支持有效的 JPEG、PNG、WebP 图片")
	}
	if !validChoice(format, "jpeg", "png", "webp") || cfg.Width <= 0 || cfg.Height <= 0 || int64(cfg.Width)*int64(cfg.Height) > 24000000 {
		return "", fmt.Errorf("图片尺寸不能超过 2400 万像素")
	}
	return format, nil
}

// UploadPhoto forwards the original bytes once. All image encoding belongs to oneimg.
func (h *ImageHost) UploadPhoto(ctx context.Context, data []byte, albumKey, catName string) (Photo, error) {
	var p Photo
	format, err := imageFormat(data)
	if err != nil {
		return p, err
	}
	ext := map[string]string{"jpeg": "jpg", "png": "png", "webp": "webp"}[format]
	name := "NTR-" + imageStem(catName) + "-" + albumKey + "-" + randomKey() + "." + ext
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if h.FolderID > 0 {
		if err := writer.WriteField("folder_id", fmt.Sprint(h.FolderID)); err != nil {
			return p, err
		}
	}
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", mime.FormatMediaType("form-data", map[string]string{"name": "images[]", "filename": name}))
	header.Set("Content-Type", "image/"+format)
	part, err := writer.CreatePart(header)
	if err != nil {
		return p, err
	}
	if _, err = part.Write(data); err != nil {
		return p, err
	}
	if err = writer.Close(); err != nil {
		return p, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.BaseURL+"/api/upload/images", &body)
	if err != nil {
		return p, err
	}
	req.Header.Set("Authorization", "Bearer "+h.Token)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	res, err := h.Client.Do(req)
	if err != nil {
		return p, fmt.Errorf("图床暂时无法连接")
	}
	defer res.Body.Close()
	var envelope struct {
		Code int `json:"code"`
		Data struct {
			Files []struct {
				ID           int64  `json:"id"`
				FolderID     int64  `json:"folder_id"`
				Success      bool   `json:"success"`
				URL          string `json:"url"`
				ThumbnailURL string `json:"thumbnail_url"`
				MimeType     string `json:"mime_type"`
				Width        int    `json:"width"`
				Height       int    `json:"height"`
			} `json:"files"`
		} `json:"data"`
	}
	if res.StatusCode != 200 {
		return p, fmt.Errorf("图床返回状态 %d，请联系管理员检查 Token 和上传设置", res.StatusCode)
	}
	if err = json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&envelope); err != nil {
		return p, fmt.Errorf("图床响应格式无效")
	}
	if envelope.Code != 200 || len(envelope.Data.Files) != 1 || !envelope.Data.Files[0].Success || envelope.Data.Files[0].ID <= 0 {
		return p, fmt.Errorf("图床未成功保存图片")
	}
	file := envelope.Data.Files[0]
	p = Photo{ImageID: file.ID, URL: file.URL, Thumbnail: file.ThumbnailURL, Width: file.Width, Height: file.Height}
	if !publicImageURL(p.URL) {
		return p, fmt.Errorf("图床未返回有效的公开图片 URL")
	}
	if h.FolderID > 0 && file.FolderID != h.FolderID {
		return p, fmt.Errorf("图床未将图片归入指定目录，请检查官方版本和目录归属")
	}
	if file.MimeType != "image/webp" || !publicImageURL(p.Thumbnail) || p.Thumbnail == p.URL {
		return p, fmt.Errorf("图床未返回 WebP 主图和独立缩略图，请在 oneimg 开启图片压缩、保存 WebP 和缩略图")
	}
	if p.Width <= 0 || p.Height <= 0 {
		return p, fmt.Errorf("图床未返回有效的图片尺寸")
	}
	return p, nil
}
func publicImageURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Host != "" && validChoice(u.Scheme, "http", "https") && u.User == nil
}

func randomKey() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		panic("secure random unavailable")
	}
	return hex.EncodeToString(b)
}
func imageStem(name string) string {
	name = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' {
			return r
		}
		return '-'
	}, name)
	r := []rune(name)
	if len(r) > 20 {
		r = r[:20]
	}
	if len(r) == 0 {
		return "cat"
	}
	return string(r)
}
