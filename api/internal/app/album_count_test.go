package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"mime/multipart"
	"net/http/httptest"
	"testing"
)

func TestCameraPhotoPixelBudget(t *testing.T) {
	for _, tc := range []struct {
		name          string
		width, height int
		wantError     bool
	}{
		{"24MP camera original", 5712, 4284, false},
		{"above 32MP budget", 6000, 6000, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var original bytes.Buffer
			if err := png.Encode(&original, image.NewGray(image.Rect(0, 0, tc.width, tc.height))); err != nil {
				t.Fatal(err)
			}
			format, err := imageFormat(original.Bytes())
			if (err != nil) != tc.wantError || (!tc.wantError && format != "png") {
				t.Fatalf("format=%q error=%v, wantError=%v", format, err, tc.wantError)
			}
		})
	}
}

func TestAlbumCanCreateAppendAndEditMoreThanSixPhotos(t *testing.T) {
	f := setup(t)
	shared := login(t, f, false, "", testAccess)
	admin := login(t, f, true, "", "")
	photo := pngBytes(t)
	response := createCat(t, f, shared, photo, photo, photo, photo, photo, photo, photo)
	if response.Code != 201 {
		t.Fatalf("create seven photos: %d %s", response.Code, response.Body.String())
	}
	var cat Cat
	if err := json.Unmarshal(response.Body.Bytes(), &cat); err != nil {
		t.Fatal(err)
	}
	if len(cat.Photos) != 7 || f.uploads.Load() != 7 {
		t.Fatal("seven original photos must be uploaded and saved")
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for range 2 {
		part, err := writer.CreateFormFile("photos", "photo.png")
		if err != nil {
			t.Fatal(err)
		}
		_, _ = part.Write(photo)
	}
	_ = writer.Close()
	r := httptest.NewRequest("POST", fmt.Sprintf("http://tnr.example.test/api/admin/cats/%d/photos", cat.ID), &body)
	r.Header.Set("Origin", "http://tnr.example.test")
	r.Header.Set("Content-Type", writer.FormDataContentType())
	r.AddCookie(admin)
	response = httptest.NewRecorder()
	f.handler.ServeHTTP(response, r)
	if response.Code != 200 {
		t.Fatalf("append beyond six: %d %s", response.Code, response.Body.String())
	}
	if err := json.Unmarshal(response.Body.Bytes(), &cat); err != nil {
		t.Fatal(err)
	}
	if len(cat.Photos) != 9 || f.uploads.Load() != 9 {
		t.Fatal("append must retain previous photos and save both additions")
	}
	cat.Description = "九张照片的完整档案"
	cat.Photos[0], cat.Photos[8] = cat.Photos[8], cat.Photos[0]
	response = request(f, "PUT", fmt.Sprintf("/api/admin/cats/%d", cat.ID), cat, admin)
	if response.Code != 200 {
		t.Fatalf("edit nine-photo album: %d %s", response.Code, response.Body.String())
	}
	var saved Cat
	response = request(f, "GET", fmt.Sprintf("/api/cats/%d", cat.ID), nil, nil)
	if err := json.Unmarshal(response.Body.Bytes(), &saved); err != nil {
		t.Fatal(err)
	}
	if len(saved.Photos) != 9 || saved.Photos[0].ImageID != cat.Photos[0].ImageID || saved.Description != cat.Description {
		t.Fatal("editing must retain all nine photos and the selected cover")
	}
	cat.Photos = nil
	response = request(f, "PUT", fmt.Sprintf("/api/admin/cats/%d", cat.ID), cat, admin)
	if response.Code != 400 {
		t.Fatal("an empty album must still be rejected")
	}
}
