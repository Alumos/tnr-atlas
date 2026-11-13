package app

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOneimgRelativeURLsSaveCat(t *testing.T) {
	for _, tc := range []struct {
		name, main, thumb, origin, wantMain, wantThumb, wantError string
	}{
		{
			name: "official local storage paths",
			main: "/uploads/2026/11/cat.webp", thumb: "/uploads/2026/11/cat_thumb.webp",
			origin:   "https://images.example.test",
			wantMain: "https://images.example.test/uploads/2026/11/cat.webp", wantThumb: "https://images.example.test/uploads/2026/11/cat_thumb.webp",
		},
		{
			name: "relative path and signed query",
			main: "uploads/cat.webp?signature=a%2Fb&expires=123", thumb: "uploads/cat_thumb.webp",
			origin:   "https://images.example.test/",
			wantMain: "https://images.example.test/uploads/cat.webp?signature=a%2Fb&expires=123", wantThumb: "https://images.example.test/uploads/cat_thumb.webp",
		},
		{
			name: "absolute CDN main with relative thumbnail",
			main: "https://cdn.example.test/cat.webp?signature=a%2Fb", thumb: "/uploads/cat_thumb.webp",
			origin:   "https://images.example.test",
			wantMain: "https://cdn.example.test/cat.webp?signature=a%2Fb", wantThumb: "https://images.example.test/uploads/cat_thumb.webp",
		},
		{
			name: "missing public origin gives actionable error",
			main: "/uploads/cat.webp", thumb: "/uploads/cat_thumb.webp",
			wantError: "ONEIMG_PUBLIC_ORIGIN",
		},
		{
			name: "same main and thumbnail after resolution rejected",
			main: "https://images.example.test/uploads/cat.webp", thumb: "/uploads/cat.webp",
			origin: "https://images.example.test", wantError: "独立缩略图",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := setup(t)
			original := pngBytes(t)
			calls := 0
			host := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != "POST" || r.URL.Path != "/api/upload/images" {
					t.Error("TNR must only upload once; it must not fetch, convert or delete the image")
				}
				if err := r.ParseMultipartForm(1 << 20); err != nil {
					t.Error(err)
					w.WriteHeader(400)
					return
				}
				defer r.MultipartForm.RemoveAll()
				file, err := r.MultipartForm.File["images[]"][0].Open()
				if err != nil {
					t.Error(err)
					w.WriteHeader(400)
					return
				}
				data, err := io.ReadAll(file)
				file.Close()
				if err != nil || !bytes.Equal(data, original) {
					t.Error("original image bytes changed")
				}
				writeJSON(w, 200, map[string]any{"code": 200, "data": map[string]any{"files": []any{map[string]any{
					"success": true, "id": 123, "folder_id": 42, "url": tc.main, "thumbnail_url": tc.thumb,
					"mime_type": "image/webp", "width": 32, "height": 24,
				}}}})
			}))
			defer host.Close()
			f.server.Host.BaseURL = host.URL
			f.server.Host.PublicOrigin = tc.origin
			cookie := login(t, f, false, "", testAccess)
			response := createCat(t, f, cookie, original)
			if calls != 1 {
				t.Fatalf("expected exactly one upload, got %d", calls)
			}
			cats, total, err := f.store.List(context.Background(), "", "", "", "", 1)
			if err != nil {
				t.Fatal(err)
			}
			if tc.wantError != "" {
				if response.Code != 502 || !strings.Contains(response.Body.String(), tc.wantError) || total != 0 {
					t.Fatalf("invalid host response published: %d %s, records=%d", response.Code, response.Body.String(), total)
				}
				admin := login(t, f, true, "", "")
				audit := request(f, "GET", "/api/admin/audit", nil, admin)
				if !strings.Contains(audit.Body.String(), "album.retained") || !strings.Contains(audit.Body.String(), tc.main) {
					t.Fatal("failed upload lost retained image audit")
				}
				return
			}
			var cat Cat
			if err := json.Unmarshal(response.Body.Bytes(), &cat); err != nil {
				t.Fatal(err)
			}
			if response.Code != 201 || total != 1 || len(cat.Photos) != 1 || len(cats[0].Photos) != 1 {
				t.Fatalf("cat not saved: %d %s", response.Code, response.Body.String())
			}
			for _, photo := range []Photo{cat.Photos[0], cats[0].Photos[0]} {
				if photo.URL != tc.wantMain || photo.Thumbnail != tc.wantThumb || photo.ImageID != 123 {
					t.Fatalf("incorrect public image metadata: %+v", photo)
				}
			}
		})
	}
}

func TestImageURLValidation(t *testing.T) {
	for _, raw := range []string{"", "javascript:alert(1)", "data:image/webp;base64,abc", "//other.test/main.webp", "https://user:password@img.test/main.webp", "https://", "/bad%zz.webp", "?query=only", "#fragment", "\\other.test\\main.webp", "/uploads/main\n.webp"} {
		t.Run(raw, func(t *testing.T) {
			if _, err := resolveImageURL(raw, "https://images.example.test"); err == nil {
				t.Fatalf("accepted invalid image URL %q", raw)
			}
		})
	}
	for _, origin := range []string{"images.example.test", "//images.example.test", "https://user:password@images.example.test", "https://images.example.test/subpath", "https://images.example.test?x=1", "https://images.example.test?", "https://images.example.test#fragment"} {
		if err := ValidateImagePublicOrigin(origin); err == nil {
			t.Errorf("accepted invalid public origin %q", origin)
		}
	}
	for _, origin := range []string{"", "https://images.example.test", "https://images.example.test/", "http://127.0.0.1:8091"} {
		if err := ValidateImagePublicOrigin(origin); err != nil {
			t.Errorf("rejected valid public origin %q: %v", origin, err)
		}
	}
	const absolute = "https://cdn.example.test/main.webp?signature=a%2Fb"
	if got, err := resolveImageURL(absolute, ""); err != nil || got != absolute {
		t.Fatalf("absolute URLs must remain compatible without public origin: %q %v", got, err)
	}
}
