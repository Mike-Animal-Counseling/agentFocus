package watcher

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHookAuthenticationAndBrowserRejection(t *testing.T) {
	h := &httpSource{addr: "127.0.0.1:27182", token: strings.Repeat("a", 64), active: make(chan struct{}, 16)}
	handler := h.protect(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
	cases := []struct {
		host, origin, token string
		status              int
	}{
		{h.addr, "", "", 401}, {"attacker.test", "", h.token, 403}, {h.addr, "http://127.0.0.1:27182", h.token, 403}, {h.addr, "", h.token, 204},
	}
	for _, item := range cases {
		request := httptest.NewRequest("POST", "http://"+item.host+"/hook", strings.NewReader("{}"))
		request.Header.Set("Origin", item.origin)
		request.Header.Set("Authorization", "Bearer "+item.token)
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != item.status {
			t.Fatalf("status %d, want %d", response.Code, item.status)
		}
	}
}
