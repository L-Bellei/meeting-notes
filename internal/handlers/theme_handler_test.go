package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"meeting-notes/internal/database"
	"meeting-notes/internal/handlers"
	"meeting-notes/internal/models"
	"meeting-notes/internal/repository"
	"meeting-notes/internal/services"
)

func newTestThemeHandler(t *testing.T) *handlers.ThemeHandler {
	t.Helper()
	db, err := database.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	repo := repository.NewThemeRepository(db)
	svc := services.NewThemeService(repo)
	return handlers.NewThemeHandler(svc)
}

func withChiID(req *http.Request, id string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

func TestThemeHandler_List_Empty(t *testing.T) {
	h := newTestThemeHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/themes", nil)
	w := httptest.NewRecorder()
	h.List(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	var result []models.Theme
	json.NewDecoder(w.Body).Decode(&result)
	if len(result) != 0 {
		t.Errorf("expected empty list, got %d", len(result))
	}
}

func TestThemeHandler_Create(t *testing.T) {
	h := newTestThemeHandler(t)

	body := `{"name":"Produto","description":"desc","color":"#8b5cf6"}`
	req := httptest.NewRequest(http.MethodPost, "/api/themes", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Create(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201; body: %s", w.Code, w.Body.String())
	}
	var theme models.Theme
	json.NewDecoder(w.Body).Decode(&theme)
	if theme.ID == "" {
		t.Error("ID should be set")
	}
	if theme.Name != "Produto" {
		t.Errorf("Name = %q", theme.Name)
	}
}

func TestThemeHandler_Create_NameRequired(t *testing.T) {
	h := newTestThemeHandler(t)

	body := `{"description":"sem nome"}`
	req := httptest.NewRequest(http.MethodPost, "/api/themes", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Create(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", w.Code)
	}
}

func TestThemeHandler_Create_Duplicate(t *testing.T) {
	h := newTestThemeHandler(t)

	body := `{"name":"Dup"}`
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/themes", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.Create(w, req)
		if i == 1 && w.Code != http.StatusConflict {
			t.Errorf("second create status = %d, want 409", w.Code)
		}
	}
}

func TestThemeHandler_GetByID(t *testing.T) {
	h := newTestThemeHandler(t)

	body := `{"name":"Eng"}`
	reqC := httptest.NewRequest(http.MethodPost, "/api/themes", bytes.NewBufferString(body))
	reqC.Header.Set("Content-Type", "application/json")
	wC := httptest.NewRecorder()
	h.Create(wC, reqC)
	var created models.Theme
	json.NewDecoder(wC.Body).Decode(&created)

	req := withChiID(httptest.NewRequest(http.MethodGet, "/api/themes/"+created.ID, nil), created.ID)
	w := httptest.NewRecorder()
	h.GetByID(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestThemeHandler_GetByID_NotFound(t *testing.T) {
	h := newTestThemeHandler(t)

	req := withChiID(httptest.NewRequest(http.MethodGet, "/api/themes/nope", nil), "nope")
	w := httptest.NewRecorder()
	h.GetByID(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestThemeHandler_Update(t *testing.T) {
	h := newTestThemeHandler(t)

	body := `{"name":"Original"}`
	reqC := httptest.NewRequest(http.MethodPost, "/api/themes", bytes.NewBufferString(body))
	reqC.Header.Set("Content-Type", "application/json")
	wC := httptest.NewRecorder()
	h.Create(wC, reqC)
	var created models.Theme
	json.NewDecoder(wC.Body).Decode(&created)

	updateBody := `{"name":"Atualizado","color":"#ff0000"}`
	req := withChiID(
		httptest.NewRequest(http.MethodPut, "/api/themes/"+created.ID, bytes.NewBufferString(updateBody)),
		created.ID,
	)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Update(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	var updated models.Theme
	json.NewDecoder(w.Body).Decode(&updated)
	if updated.Name != "Atualizado" {
		t.Errorf("Name = %q", updated.Name)
	}
}

func TestThemeHandler_Delete(t *testing.T) {
	h := newTestThemeHandler(t)

	body := `{"name":"Para deletar"}`
	reqC := httptest.NewRequest(http.MethodPost, "/api/themes", bytes.NewBufferString(body))
	reqC.Header.Set("Content-Type", "application/json")
	wC := httptest.NewRecorder()
	h.Create(wC, reqC)
	var created models.Theme
	json.NewDecoder(wC.Body).Decode(&created)

	req := withChiID(
		httptest.NewRequest(http.MethodDelete, "/api/themes/"+created.ID, nil),
		created.ID,
	)
	w := httptest.NewRecorder()
	h.Delete(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", w.Code)
	}
}

func TestThemeHandler_Delete_NotFound(t *testing.T) {
	h := newTestThemeHandler(t)

	req := withChiID(httptest.NewRequest(http.MethodDelete, "/api/themes/nope", nil), "nope")
	w := httptest.NewRecorder()
	h.Delete(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}
