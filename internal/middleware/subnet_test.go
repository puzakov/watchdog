package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCheckSubnet_EmptySubnet_Noop(t *testing.T) {
	var called bool
	h := CheckSubnet("", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if !called {
		t.Error("handler should be called when subnet is empty")
	}
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestCheckSubnet_ValidIP_Allowed(t *testing.T) {
	var called bool
	h := CheckSubnet("192.168.1.0/24", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Real-IP", "192.168.1.100")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if !called {
		t.Error("handler should be called for IP in trusted subnet")
	}
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestCheckSubnet_InvalidIP_Forbidden(t *testing.T) {
	var called bool
	h := CheckSubnet("192.168.1.0/24", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Real-IP", "10.0.0.1")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if called {
		t.Error("handler should NOT be called for IP outside trusted subnet")
	}
	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestCheckSubnet_MissingHeader_Forbidden(t *testing.T) {
	var called bool
	h := CheckSubnet("192.168.1.0/24", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	// No X-Real-IP header set.
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if called {
		t.Error("handler should NOT be called when X-Real-IP is missing")
	}
	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestCheckSubnet_InvalidIPString_Forbidden(t *testing.T) {
	var called bool
	h := CheckSubnet("192.168.1.0/24", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Real-IP", "not-an-ip")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if called {
		t.Error("handler should NOT be called when X-Real-IP is invalid")
	}
	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestCheckSubnet_IPv6InSubnet_Allowed(t *testing.T) {
	var called bool
	h := CheckSubnet("2001:db8::/32", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Real-IP", "2001:db8::1")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if !called {
		t.Error("handler should be called for IPv6 in trusted subnet")
	}
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestCheckSubnet_InvalidCIDR_InternalServerError(t *testing.T) {
	var called bool
	h := CheckSubnet("not-a-cidr", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Real-IP", "192.168.1.1")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if called {
		t.Error("handler should NOT be called when CIDR is invalid")
	}
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}
