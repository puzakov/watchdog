package middleware

import (
	"bytes"
	"io"
	"net/http"

	"github.com/puzakov/watchdog/internal/sign"
)

func HashSHA256(key string, next http.Handler) http.Handler {
	if key == "" {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := r.Header.Get(sign.HeaderHashSHA256)
		if got != "" && r.Body != nil {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			_ = r.Body.Close()

			want := sign.SumSHA256(body, key)
			if got != want {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			r.Body = io.NopCloser(bytes.NewReader(body))
		}

		brw := newBufferedResponseWriter(w)
		next.ServeHTTP(brw, r)

		brw.header.Set(sign.HeaderHashSHA256, sign.SumSHA256(brw.body.Bytes(), key))
		brw.flush()
	})
}

type bufferedResponseWriter struct {
	underlying http.ResponseWriter
	header     http.Header
	body       bytes.Buffer
	status     int
}

func newBufferedResponseWriter(w http.ResponseWriter) *bufferedResponseWriter {
	return &bufferedResponseWriter{
		underlying: w,
		header:     make(http.Header),
		status:     http.StatusOK,
	}
}

func (b *bufferedResponseWriter) Header() http.Header {
	return b.header
}

func (b *bufferedResponseWriter) WriteHeader(statusCode int) {
	b.status = statusCode
}

func (b *bufferedResponseWriter) Write(p []byte) (int, error) {
	return b.body.Write(p)
}

func (b *bufferedResponseWriter) flush() {
	dst := b.underlying.Header()
	for k, vv := range b.header {
		dst[k] = append([]string(nil), vv...)
	}
	b.underlying.WriteHeader(b.status)
	_, _ = b.underlying.Write(b.body.Bytes())
}
