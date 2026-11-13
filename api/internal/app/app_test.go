package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

const testAccess = "test-access-code-123456"
const testAdmin = "test-admin-password-123456"

type fixture struct {
	server    *Server
	handler   http.Handler
	store     *Store
	uploads   atomic.Int64
	imageHost *httptest.Server
}

func setup(t *testing.T) *fixture {
	t.Helper()
	store, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err = store.InitAuth(testAccess); err != nil {
		t.Fatal(err)
	}
	f := &fixture{store: store}
	f.imageHost = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/api/upload/images" {
			t.Errorf("unexpected image host request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(405)
			return
		}
		if r.Header.Get("Authorization") != "Bearer test-image-token" {
			t.Error("missing server-side token")
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Error(err)
			return
		}
		defer r.MultipartForm.RemoveAll()
		if r.FormValue("folder_id") != "42" {
			t.Error("configured folder not applied")
		}
		files := r.MultipartForm.File["images[]"]
		if len(files) != 1 {
			t.Error("wrong multipart file field")
			return
		}
		if !strings.HasPrefix(files[0].Filename, "NTR-小橘-") {
			t.Errorf("wrong archive filename %q", files[0].Filename)
		}
		src, err := files[0].Open()
		if err != nil {
			t.Fatal(err)
		}
		original, err := io.ReadAll(src)
		src.Close()
		if err != nil || !bytes.Equal(original, pngBytes(t)) || !strings.HasSuffix(files[0].Filename, ".png") {
			t.Error("TNR must forward original PNG bytes with their actual extension")
		}
		id := f.uploads.Add(1)
		writeJSON(w, 200, map[string]any{"code": 200, "data": map[string]any{"files": []any{map[string]any{"success": true, "id": id, "folder_id": 42, "url": fmt.Sprintf("https://images.example.test/%d.webp", id), "thumbnail_url": fmt.Sprintf("https://images.example.test/%d-thumb.webp", id), "mime_type": "image/webp", "width": 32, "height": 24}}}})
	}))
	host := NewImageHost(f.imageHost.URL, "test-image-token")
	host.FolderID = 42
	f.server = NewServer(store, host, "", "http://tnr.example.test")
	f.server.AdminPassword = testAdmin
	f.handler = f.server.Handler()
	t.Cleanup(func() { f.imageHost.Close(); store.Close() })
	return f
}
func request(f *fixture, method, path string, body any, cookie *http.Cookie) *httptest.ResponseRecorder {
	var reader io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reader = bytes.NewReader(b)
	}
	r := httptest.NewRequest(method, "http://tnr.example.test"+path, reader)
	r.Header.Set("Origin", "http://tnr.example.test")
	if body != nil {
		r.Header.Set("Content-Type", "application/json")
	}
	if cookie != nil {
		r.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	f.handler.ServeHTTP(w, r)
	return w
}
func login(t *testing.T, f *fixture, admin bool, username, code string) *http.Cookie {
	t.Helper()
	path := "/api/session"
	body := map[string]string{"username": username, "code": code}
	if admin {
		path = "/api/admin/session"
		body = map[string]string{"password": testAdmin}
	}
	w := request(f, "POST", path, body, nil)
	if w.Code != 200 {
		t.Fatalf("login failed: %d %s", w.Code, w.Body.String())
	}
	return w.Result().Cookies()[0]
}
func pngBytes(t *testing.T) []byte {
	t.Helper()
	im := image.NewNRGBA(image.Rect(0, 0, 32, 24))
	for y := 0; y < 24; y++ {
		for x := 0; x < 32; x++ {
			im.Set(x, y, color.NRGBA{uint8(x * 5), uint8(y * 5), 100, 255})
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, im); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}
func createCat(t *testing.T, f *fixture, cookie *http.Cookie, photos ...[]byte) *httptest.ResponseRecorder {
	return createCatFields(t, f, cookie, nil, photos...)
}
func createCatFields(t *testing.T, f *fixture, cookie *http.Cookie, fields map[string]string, photos ...[]byte) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	defaults := map[string]string{"name": "小橘", "sex": "公", "color": "橘猫", "neutered": "未绝育", "description": "社区猫咪", "uploader": "试图冒用的昵称"}
	for k, v := range fields {
		defaults[k] = v
	}
	for k, v := range defaults {
		_ = writer.WriteField(k, v)
	}
	for _, photo := range photos {
		part, _ := writer.CreateFormFile("photos", "photo.png")
		_, _ = part.Write(photo)
	}
	writer.Close()
	r := httptest.NewRequest("POST", "http://tnr.example.test/api/cats", &body)
	r.Header.Set("Origin", "http://tnr.example.test")
	r.Header.Set("Content-Type", writer.FormDataContentType())
	if cookie != nil {
		r.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	f.handler.ServeHTTP(w, r)
	return w
}
func TestRegistryUploadAdminAndImageRetention(t *testing.T) {
	f := setup(t)
	if w := request(f, "GET", "/api/stats", nil, nil); w.Code != 200 {
		t.Fatal(w.Code)
	}
	if w := createCat(t, f, nil, pngBytes(t)); w.Code != 401 {
		t.Fatal("anonymous write accepted")
	}
	admin := login(t, f, true, "", "")
	v := map[string]any{"username": "xiaolin", "name": "小林", "code": "volunteer-code-123456", "active": true}
	if w := request(f, "POST", "/api/admin/volunteers", v, admin); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	volunteer := login(t, f, false, "xiaolin", "volunteer-code-123456")
	w := createCat(t, f, volunteer, pngBytes(t))
	if w.Code != 201 {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	var cat Cat
	if err := json.Unmarshal(w.Body.Bytes(), &cat); err != nil {
		t.Fatal(err)
	}
	if cat.Uploader != "小林" || cat.AlbumKey == "" || len(cat.Photos) != 1 || f.uploads.Load() != 1 || cat.Photos[0].Thumbnail != "https://images.example.test/1-thumb.webp" {
		t.Fatalf("bad creation: %+v", cat)
	}
	if w = request(f, "PUT", fmt.Sprintf("/api/admin/cats/%d", cat.ID), cat, volunteer); w.Code != 401 {
		t.Fatal("volunteer got admin privileges")
	}
	cat.Neutered = "已绝育"
	cat.Name = "小橘改名"
	if w = request(f, "PUT", fmt.Sprintf("/api/admin/cats/%d", cat.ID), cat, admin); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	if w = request(f, "GET", "/api/cats?status=已绝育", nil, nil); w.Code != 200 || !strings.Contains(w.Body.String(), "小橘改名") {
		t.Fatal(w.Body.String())
	}
	if w = request(f, "DELETE", fmt.Sprintf("/api/admin/cats/%d", cat.ID), nil, admin); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	if f.uploads.Load() != 1 {
		t.Fatal("deletion touched official image host")
	}
	if w = request(f, "GET", fmt.Sprintf("/api/cats/%d", cat.ID), nil, nil); w.Code != 404 {
		t.Fatal("deleted cat still visible")
	}
	w = request(f, "GET", "/api/admin/audit", nil, admin)
	if !strings.Contains(w.Body.String(), "cat.delete") || !strings.Contains(w.Body.String(), "cat.update") {
		t.Fatal("missing audit history")
	}
	if strings.Contains(w.Body.String(), "volunteer-code") {
		t.Fatal("audit exposed code")
	}
}
func TestAccessRotationRevokesSharedAndDisabledVolunteer(t *testing.T) {
	f := setup(t)
	admin := login(t, f, true, "", "")
	shared := login(t, f, false, "", testAccess)
	w := request(f, "PUT", "/api/admin/settings", map[string]any{"code": "replacement-access-code-123456", "sharedEnabled": true}, admin)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	if w = request(f, "GET", "/api/session", nil, shared); strings.Contains(w.Body.String(), `"authenticated":true`) {
		t.Fatal("old shared session survived rotation")
	}
	if w = request(f, "POST", "/api/session", map[string]string{"code": testAccess}, nil); w.Code != 401 {
		t.Fatal("old code accepted")
	}
	_ = login(t, f, false, "", "replacement-access-code-123456")
	v := map[string]any{"username": "helper", "name": "志愿者", "code": "test-helper-code-123456", "active": true}
	if w = request(f, "POST", "/api/admin/volunteers", v, admin); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	cookie := login(t, f, false, "helper", "test-helper-code-123456")
	v["active"] = false
	v["code"] = ""
	if w = request(f, "PUT", "/api/admin/volunteers/1", v, admin); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	if w = request(f, "GET", "/api/session", nil, cookie); strings.Contains(w.Body.String(), `"authenticated":true`) {
		t.Fatal("disabled volunteer still logged in")
	}
}
func TestRejectInvalidImageAndCrossOrigin(t *testing.T) {
	f := setup(t)
	shared := login(t, f, false, "", testAccess)
	w := createCat(t, f, shared, []byte("not an image"))
	if w.Code != 400 || f.uploads.Load() != 0 {
		t.Fatal("invalid image reached host")
	}
	r := httptest.NewRequest("PUT", "http://tnr.example.test/api/admin/settings", strings.NewReader(`{}`))
	r.Header.Set("Origin", "https://evil.example")
	w = httptest.NewRecorder()
	f.handler.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("cross-origin write accepted")
	}
	cat := Cat{Name: " ", Sex: "公", Color: "橘猫", Neutered: "未知", Uploader: "小林"}
	if cat.Validate() == nil {
		t.Fatal("empty name accepted")
	}
	cat.Name = "猫"
	cat.Color = "bad"
	if cat.Validate() == nil {
		t.Fatal("invalid color accepted")
	}
}
func TestSearchEscaping(t *testing.T) {
	f := setup(t)
	c := Cat{AlbumKey: "key", Name: "100%小猫", Sex: "未知", Color: "无法定义", Neutered: "未知", Uploader: "小林", Photos: []Photo{{URL: "https://img.test/1.webp"}}}
	if err := f.store.Create(context.Background(), &c); err != nil {
		t.Fatal(err)
	}
	cats, total, err := f.store.List(context.Background(), "%", "", "", "", 1)
	if err != nil || total != 1 || len(cats) != 1 {
		t.Fatal("literal search escaping failed", err, total)
	}
	_, total, err = f.store.List(context.Background(), "_", "", "", "", 1)
	if err != nil || total != 0 {
		t.Fatal("wildcard escaped incorrectly", err, total)
	}
}
func TestFrontendSPAFallbackAndCaching(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html>test</html>"), 0600); err != nil {
		t.Fatal(err)
	}
	handler := Frontend(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(418) }), dir)
	for _, test := range []struct {
		path   string
		status int
	}{{"/admin", 200}, {"/api/missing", 418}, {"/assets/missing.js", 404}} {
		r := httptest.NewRequest("GET", test.path, nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != test.status {
			t.Errorf("%s: %d", test.path, w.Code)
		}
	}
}

func TestOriginalJPEGForwardingAndHostContract(t *testing.T) {
	var original bytes.Buffer
	if err := jpeg.Encode(&original, image.NewNRGBA(image.Rect(0, 0, 48, 32)), nil); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, mime, thumb string
		folder            int64
		wantError         bool
	}{
		{"valid", "image/webp", "https://img.test/thumb.webp", 42, false},
		{"conversion disabled", "image/jpeg", "https://img.test/thumb.webp", 42, true},
		{"thumbnail disabled", "image/webp", "", 42, true},
		{"thumbnail unsafe", "image/webp", "javascript:alert(1)", 42, true},
		{"wrong folder", "image/webp", "https://img.test/thumb.webp", 43, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			host := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if err := r.ParseMultipartForm(1 << 20); err != nil {
					t.Error(err)
					w.WriteHeader(400)
					return
				}
				defer r.MultipartForm.RemoveAll()
				file := r.MultipartForm.File["images[]"][0]
				src, err := file.Open()
				if err != nil {
					t.Error(err)
					w.WriteHeader(400)
					return
				}
				data, err := io.ReadAll(src)
				src.Close()
				if err != nil || !bytes.Equal(data, original.Bytes()) || file.Header.Get("Content-Type") != "image/jpeg" || !strings.HasSuffix(file.Filename, ".jpg") {
					t.Error("original JPEG was altered or mislabeled")
				}
				writeJSON(w, 200, map[string]any{"code": 200, "data": map[string]any{"files": []any{map[string]any{"success": true, "id": 7, "folder_id": tc.folder, "url": "https://img.test/main.webp", "thumbnail_url": tc.thumb, "mime_type": tc.mime, "width": 48, "height": 32}}}})
			}))
			defer host.Close()
			client := NewImageHost(host.URL, "dummy-token")
			client.FolderID = 42
			p, err := client.UploadPhoto(context.Background(), original.Bytes(), "stable-album", "小橘")
			if (err != nil) != tc.wantError || calls != 1 || p.ImageID != 7 {
				t.Fatalf("contract result: calls=%d photo=%+v err=%v", calls, p, err)
			}
			if !tc.wantError && (p.Thumbnail != tc.thumb || p.URL != "https://img.test/main.webp" || p.Width != 48) {
				t.Fatal("ignored host image metadata")
			}
		})
	}
}

func TestUploadFailureRetainsArtifactsWithoutPublishing(t *testing.T) {
	f := setup(t)
	cookie := login(t, f, false, "", testAccess)
	var count int
	host := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count++
		if count == 1 {
			writeJSON(w, 200, map[string]any{"code": 200, "data": map[string]any{"files": []any{map[string]any{"success": true, "id": 99, "folder_id": 42, "url": "https://images.example.test/retained.webp", "thumbnail_url": "https://images.example.test/retained-thumb.webp", "mime_type": "image/webp", "width": 32, "height": 24}}}})
		} else {
			w.WriteHeader(503)
		}
	}))
	defer host.Close()
	f.server.Host.BaseURL = host.URL
	w := createCat(t, f, cookie, pngBytes(t), pngBytes(t))
	if w.Code != 502 {
		t.Fatalf("expected upload failure, got %d", w.Code)
	}
	stats, err := f.store.Stats(context.Background())
	if err != nil || stats.Total != 0 {
		t.Fatal("partial upload published a cat", err)
	}
	var detail string
	if err = f.store.db.QueryRow("SELECT details FROM audit WHERE action='album.retained'").Scan(&detail); err != nil || !strings.Contains(detail, "retained.webp") {
		t.Fatal("retained artifact missing from audit", err)
	}
}

func TestAdminPhotoAppendAndRemoval(t *testing.T) {
	f := setup(t)
	shared := login(t, f, false, "", testAccess)
	admin := login(t, f, true, "", "")
	w := createCat(t, f, shared, pngBytes(t))
	var cat Cat
	if w.Code != 201 {
		t.Fatal(w.Body.String())
	}
	_ = json.Unmarshal(w.Body.Bytes(), &cat)
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, _ := writer.CreateFormFile("photos", "new.png")
	_, _ = part.Write(pngBytes(t))
	writer.Close()
	r := httptest.NewRequest("POST", fmt.Sprintf("http://tnr.example.test/api/admin/cats/%d/photos", cat.ID), &body)
	r.AddCookie(admin)
	r.Header.Set("Content-Type", writer.FormDataContentType())
	r.Header.Set("Origin", "http://tnr.example.test")
	w = httptest.NewRecorder()
	f.handler.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	_ = json.Unmarshal(w.Body.Bytes(), &cat)
	if len(cat.Photos) != 2 {
		t.Fatal("append failed")
	}
	cat.Photos = cat.Photos[1:]
	w = request(f, "PUT", fmt.Sprintf("/api/admin/cats/%d", cat.ID), cat, admin)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	saved, err := f.store.Get(context.Background(), cat.ID)
	if err != nil || len(saved.Photos) != 1 || f.uploads.Load() != 2 {
		t.Fatal("photo removal or retention failed", err)
	}
}

func TestAuthPersistenceAndGzipNegotiation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "persist.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.InitAuth(testAccess); err != nil {
		t.Fatal(err)
	}
	before, _ := s.setting(context.Background(), "access_hash")
	s.Close()
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err = s.InitAuth("different-initial-code"); err != nil {
		t.Fatal(err)
	}
	after, _ := s.setting(context.Background(), "access_hash")
	if before != after {
		t.Fatal("restart overwrote persisted access code")
	}
	for _, c := range []struct {
		h    string
		want bool
	}{{"gzip, deflate", true}, {"gzip;q=0", false}, {"br", false}, {"gzip;q=0.5", true}} {
		if acceptsGzip(c.h) != c.want {
			t.Errorf("bad gzip negotiation: %s", c.h)
		}
	}
}
