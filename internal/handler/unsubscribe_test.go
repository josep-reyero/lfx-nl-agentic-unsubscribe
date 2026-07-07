// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/linuxfoundation/lfx-v2-newsletter-service/internal/service"
)

type stubUnsubRepo struct {
	created []string
}

func (s *stubUnsubRepo) CreateUnsubscribe(_ context.Context, projectUID, emailHash string) error {
	s.created = append(s.created, projectUID+"|"+emailHash)
	return nil
}
func (s *stubUnsubRepo) ListUnsubscribedHashes(_ context.Context, _ string) (map[string]struct{}, error) {
	return map[string]struct{}{}, nil
}

type stubProjectClient struct{}

func (stubProjectClient) Name(_ context.Context, _ string) (string, error) { return "CNCF", nil }
func (stubProjectClient) Slug(_ context.Context, _ string) (string, error) { return "cncf", nil }

func TestUnsubscribeHandlerGETRendersConfirmFormWithoutWriting(t *testing.T) {
	repo := &stubUnsubRepo{}
	unsub := service.NewUnsubscribeService(repo, []byte("k"), "http://localhost")
	h := &Handler{unsub: unsub, project: stubProjectClient{}}

	link := unsub.BuildURL("proj-1", "alice@example.com")
	token := link[strings.Index(link, "?t=")+3:]

	req := httptest.NewRequest(http.MethodGet, "/newsletters/unsubscribe?t="+token, nil)
	w := httptest.NewRecorder()
	h.Unsubscribe(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200. body=%s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("content-type = %q, want text/html", ct)
	}
	body := w.Body.String()
	if !strings.Contains(body, `method="post"`) || !strings.Contains(body, `name="t"`) {
		t.Errorf("body should contain the POST confirmation form: %s", body)
	}
	if !strings.Contains(body, "CNCF") {
		t.Errorf("body missing project name: %s", body)
	}
	if strings.Contains(body, "alice@example.com") {
		t.Errorf("body must not echo the plaintext address: %s", body)
	}
	// GET must be side-effect free: scanners and link-preview bots prefetch it.
	if len(repo.created) != 0 {
		t.Errorf("GET recorded an opt-out: %v", repo.created)
	}
}

func TestUnsubscribeHandlerPOSTRecordsOptOut(t *testing.T) {
	repo := &stubUnsubRepo{}
	unsub := service.NewUnsubscribeService(repo, []byte("k"), "http://localhost")
	h := &Handler{unsub: unsub, project: stubProjectClient{}}

	link := unsub.BuildURL("proj-1", "alice@example.com")
	token := link[strings.Index(link, "?t=")+3:]

	form := url.Values{"t": {token}}
	req := httptest.NewRequest(http.MethodPost, "/newsletters/unsubscribe", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.ConfirmUnsubscribe(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200. body=%s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if !strings.Contains(body, "CNCF") || !strings.Contains(body, "no longer receive") {
		t.Errorf("body missing confirmation copy: %s", body)
	}
	if strings.Contains(body, "alice@example.com") {
		t.Errorf("body must not echo the plaintext address: %s", body)
	}
	want := "proj-1|" + service.HashRecipient("alice@example.com")
	if len(repo.created) != 1 || repo.created[0] != want {
		t.Errorf("repo.created = %v, want [%s]", repo.created, want)
	}
}

func TestUnsubscribeHandlerInvalidToken(t *testing.T) {
	repo := &stubUnsubRepo{}
	unsub := service.NewUnsubscribeService(repo, []byte("k"), "http://localhost")
	h := &Handler{unsub: unsub, project: stubProjectClient{}}

	req := httptest.NewRequest(http.MethodGet, "/newsletters/unsubscribe?t=garbage", nil)
	w := httptest.NewRecorder()
	h.Unsubscribe(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("GET status = %d, want 400", w.Code)
	}
	if !strings.Contains(w.Body.String(), "invalid") && !strings.Contains(w.Body.String(), "Invalid") {
		t.Errorf("body should mention invalid link: %s", w.Body.String())
	}

	form := url.Values{"t": {"garbage"}}
	req = httptest.NewRequest(http.MethodPost, "/newsletters/unsubscribe", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w = httptest.NewRecorder()
	h.ConfirmUnsubscribe(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("POST status = %d, want 400", w.Code)
	}
	if len(repo.created) != 0 {
		t.Errorf("invalid token recorded an opt-out: %v", repo.created)
	}
}
