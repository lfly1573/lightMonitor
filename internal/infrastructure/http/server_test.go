package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"lightmonitor/internal/application/install"
	"lightmonitor/internal/domain/system"
)

type fakeInstallRepository struct {
	installed bool
}

func (r *fakeInstallRepository) IsInstalled(ctx context.Context) (bool, error) {
	_ = ctx
	return r.installed, nil
}

func (r *fakeInstallRepository) Install(ctx context.Context, admin system.User) error {
	_ = ctx
	_ = admin
	r.installed = true
	return nil
}

func TestServerRoutes(t *testing.T) {
	server := NewServer(Dependencies{
		InstallService: install.NewService(&fakeInstallRepository{}),
	})

	tests := []struct {
		name       string
		path       string
		wantStatus int
	}{
		{name: "health", path: "/api/health", wantStatus: http.StatusOK},
		{name: "install status", path: "/api/install/status", wantStatus: http.StatusOK},
		{name: "front end index", path: "/", wantStatus: http.StatusOK},
		{name: "install page fallback", path: "/install", wantStatus: http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()

			server.router.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
		})
	}
}
