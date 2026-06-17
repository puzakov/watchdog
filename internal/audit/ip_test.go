package audit

import (
	"net/http"
	"testing"
)

func TestClientIP_XForwardedFor(t *testing.T) {
	r := &http.Request{
		Header: http.Header{
			"X-Forwarded-For": {"10.0.0.1, 10.0.0.2"},
		},
	}
	if ip := ClientIP(r); ip != "10.0.0.1" {
		t.Errorf("ClientIP = %q, want %q", ip, "10.0.0.1")
	}
}

func TestClientIP_XRealIP(t *testing.T) {
	r := &http.Request{
		Header: http.Header{
			"X-Real-Ip": {"10.0.0.3"},
		},
	}
	if ip := ClientIP(r); ip != "10.0.0.3" {
		t.Errorf("ClientIP = %q, want %q", ip, "10.0.0.3")
	}
}

func TestClientIP_XForwardedForTakesPrecedence(t *testing.T) {
	r := &http.Request{
		Header: http.Header{
			"X-Forwarded-For": {"10.0.0.1"},
			"X-Real-Ip":       {"10.0.0.3"},
		},
	}
	if ip := ClientIP(r); ip != "10.0.0.1" {
		t.Errorf("ClientIP = %q, want %q", ip, "10.0.0.1")
	}
}

func TestClientIP_RemoteAddr(t *testing.T) {
	r := &http.Request{
		RemoteAddr: "192.168.1.1:12345",
	}
	if ip := ClientIP(r); ip != "192.168.1.1" {
		t.Errorf("ClientIP = %q, want %q", ip, "192.168.1.1")
	}
}

func TestClientIP_RemoteAddrNoPort(t *testing.T) {
	r := &http.Request{
		RemoteAddr: "192.168.1.1",
	}
	if ip := ClientIP(r); ip != "192.168.1.1" {
		t.Errorf("ClientIP = %q, want %q", ip, "192.168.1.1")
	}
}

func TestClientIP_Empty(t *testing.T) {
	r := &http.Request{}
	if ip := ClientIP(r); ip != "" {
		t.Errorf("ClientIP = %q, want empty string", ip)
	}
}
