package debug

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMux(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		path       string
		wantStatus int
	}{
		{
			name:       "pprof index",
			path:       "/debug/pprof/",
			wantStatus: http.StatusOK,
		},
		{
			name:       "pprof cmdline",
			path:       "/debug/pprof/cmdline",
			wantStatus: http.StatusOK,
		},
		{
			name:       "pprof symbol",
			path:       "/debug/pprof/symbol",
			wantStatus: http.StatusOK,
		},
		{
			name:       "expvar metrics",
			path:       "/debug/vars/",
			wantStatus: http.StatusOK,
		},
	}

	mux := Mux()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			w := httptest.NewRecorder()

			mux.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d", tt.wantStatus, w.Code)
			}
		})
	}
}
