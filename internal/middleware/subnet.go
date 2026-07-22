package middleware

import (
	"net"
	"net/http"

	"github.com/puzakov/watchdog/internal/logger"
	"go.uber.org/zap"
)

// CheckSubnet is HTTP middleware that validates the X-Real-IP header
// against a trusted subnet (CIDR). If trustedSubnet is empty, the middleware
// is a no-op. Otherwise, requests from IPs outside the subnet receive a
// 403 Forbidden response.
func CheckSubnet(trustedSubnet string, next http.Handler) http.Handler {
	if trustedSubnet == "" {
		return next
	}

	_, cidr, err := net.ParseCIDR(trustedSubnet)
	if err != nil {
		logger.Log.Error("invalid trusted subnet", zap.String("subnet", trustedSubnet), zap.Error(err))
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		})
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ipStr := r.Header.Get("X-Real-IP")
		if ipStr == "" {
			logger.Log.Warn("request missing X-Real-IP header")
			w.WriteHeader(http.StatusForbidden)
			return
		}

		ip := net.ParseIP(ipStr)
		if ip == nil {
			logger.Log.Warn("invalid X-Real-IP", zap.String("ip", ipStr))
			w.WriteHeader(http.StatusForbidden)
			return
		}

		if !cidr.Contains(ip) {
			logger.Log.Warn("request from untrusted IP",
				zap.String("ip", ipStr),
				zap.String("trusted_subnet", trustedSubnet),
			)
			w.WriteHeader(http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}
