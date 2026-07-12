// SPDX-License-Identifier: Apache-2.0
package health

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/FreshLabDev/makeitMD/internal/db"
)

type fakeStore struct{ err error }

func (f fakeStore) HealthStatus(context.Context) (db.HealthStatus, error) {
	return db.HealthStatus{Received: 2, Failed: 1}, f.err
}

func TestHealthy(t *testing.T) {
	handler := New(fakeStore{}, time.Now, time.Now(), Build{Version: "test"}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestUnhealthyDatabase(t *testing.T) {
	handler := New(fakeStore{err: errors.New("down")}, time.Now, time.Now(), Build{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d", recorder.Code)
	}
}
