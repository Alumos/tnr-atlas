package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func TestNeuteredDateMigrationPreservesExistingData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE cats(id INTEGER PRIMARY KEY AUTOINCREMENT,name TEXT NOT NULL,sex TEXT NOT NULL,color TEXT NOT NULL,neutered TEXT NOT NULL,description TEXT NOT NULL,uploader TEXT NOT NULL,photos TEXT NOT NULL,created_at TEXT NOT NULL,album_key TEXT NOT NULL DEFAULT '');
	INSERT INTO cats VALUES(7,'旧档案','母','橘猫','已绝育','历史描述','志愿者','[{"imageId":99,"url":"https://img.test/cat.webp","thumbnail":"https://img.test/thumb.webp","width":32,"height":24}]','2026-11-01T00:00:00Z','original-album');`)
	db.Close()
	if err != nil {
		t.Fatal(err)
	}
	for attempt := range 2 {
		store, err := Open(path)
		if err != nil {
			t.Fatalf("migration/reopen %d: %v", attempt, err)
		}
		cat, err := store.Get(context.Background(), 7)
		if err != nil || cat.NeuteredAt != "" || cat.Name != "旧档案" || cat.AlbumKey != "original-album" || len(cat.Photos) != 1 || cat.Photos[0].ImageID != 99 || cat.CreatedAt != "2026-11-01T00:00:00Z" {
			t.Fatalf("legacy data changed: %+v %v", cat, err)
		}
		if err := store.InitAuth(testAccess); err != nil {
			t.Fatal(err)
		}
		if attempt == 0 {
			_, err = store.db.Exec("INSERT INTO audit(actor,action,target,details,created_at) VALUES('管理员','migration.test','7','{}','2026-11-01T00:00:00Z')")
			if err != nil {
				t.Fatal(err)
			}
		} else {
			var count int
			if err := store.db.QueryRow("SELECT COUNT(*) FROM audit WHERE action='migration.test'").Scan(&count); err != nil || count != 1 {
				t.Fatal("audit lost on reopening", count, err)
			}
		}
		store.Close()
	}
}

func TestNeuteredDateCreateEditClearAndAudit(t *testing.T) {
	f := setup(t)
	cookie := login(t, f, false, "", testAccess)
	response := createCatFields(t, f, cookie, map[string]string{"neutered": "已绝育", "neuteredAt": "2024-02-29"}, pngBytes(t))
	if response.Code != 201 {
		t.Fatalf("create: %d %s", response.Code, response.Body.String())
	}
	var cat Cat
	if err := json.Unmarshal(response.Body.Bytes(), &cat); err != nil || cat.NeuteredAt != "2024-02-29" {
		t.Fatal("date missing in creation response", cat, err)
	}
	admin := login(t, f, true, "", "")
	path := fmt.Sprintf("/api/admin/cats/%d", cat.ID)
	cat.NeuteredAt = "2026-11-12"
	if response = request(f, "PUT", path, cat, admin); response.Code != 200 {
		t.Fatal(response.Body.String())
	}
	stored, err := f.store.Get(context.Background(), cat.ID)
	if err != nil || stored.NeuteredAt != cat.NeuteredAt {
		t.Fatal("date not persisted", stored, err)
	}
	list := request(f, "GET", "/api/cats", nil, nil)
	if !strings.Contains(list.Body.String(), `"neuteredAt":"2026-11-12"`) {
		t.Fatal("date missing from public records")
	}
	cat.NeuteredAt = "2025-02-29"
	if response = request(f, "PUT", path, cat, admin); response.Code != 400 {
		t.Fatal("invalid calendar date accepted")
	}
	cat.NeuteredAt = ""
	cat.Neutered = "未知"
	if response = request(f, "PUT", path, cat, admin); response.Code != 200 {
		t.Fatal("could not clear date", response.Body.String())
	}
	stored, err = f.store.Get(context.Background(), cat.ID)
	if err != nil || stored.NeuteredAt != "" || stored.Neutered != "未知" {
		t.Fatal("cleared date not saved", stored, err)
	}
	audit := request(f, "GET", "/api/admin/audit", nil, admin)
	for _, expected := range []string{"2024-02-29", "2026-11-12", "neuteredAt"} {
		if !strings.Contains(audit.Body.String(), expected) {
			t.Fatal("date change absent in audit", expected)
		}
	}
}

func TestInvalidNeuteredDateRejectedBeforeUpload(t *testing.T) {
	f := setup(t)
	cookie := login(t, f, false, "", testAccess)
	for _, fields := range []map[string]string{
		{"neutered": "已绝育", "neuteredAt": "2026-02-30"},
		{"neutered": "已绝育", "neuteredAt": "2026-1-2"},
		{"neutered": "已绝育", "neuteredAt": "0000-01-01"},
		{"neutered": "已绝育", "neuteredAt": "2026-11-12T10:30:00Z"},
		{"neutered": "未绝育", "neuteredAt": "2026-11-12"},
		{"neutered": "未知", "neuteredAt": "2026-11-12"},
	} {
		response := createCatFields(t, f, cookie, fields, pngBytes(t))
		if response.Code != 400 || f.uploads.Load() != 0 {
			t.Fatalf("invalid date reached host: fields=%v status=%d uploads=%d", fields, response.Code, f.uploads.Load())
		}
	}
	if response := createCatFields(t, f, cookie, map[string]string{"neutered": "已绝育", "neuteredAt": ""}, pngBytes(t)); response.Code != 201 {
		t.Fatal("unknown date must remain optional", response.Body.String())
	}
}
