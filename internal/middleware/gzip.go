package middleware

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
)

type gzipResponseWriter struct {
	http.ResponseWriter
	Writer   io.Writer
	compress bool
	isInit   bool
}

func (w *gzipResponseWriter) Write(b []byte) (int, error) {
	if !w.isInit {
		ct := w.Header().Get("Content-Type")
		if strings.HasPrefix(ct, "application/json") || strings.HasPrefix(ct, "text/html") {
			w.compress = true
			w.Header().Set("Content-Encoding", "gzip")
			w.Header().Add("Vary", "Accept-Encoding")
		}
		w.isInit = true
	}

	if w.compress {
		return w.Writer.Write(b)
	}

	return w.ResponseWriter.Write(b)
}

func Gzip(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if shouldDecompressRequest(r) {
			gzr, err := gzip.NewReader(r.Body)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			defer gzr.Close()
			r.Body = io.NopCloser(gzr)
			r.Header.Del("Content-Encoding")
		}

		if !clientAcceptsGzip(r) {
			next.ServeHTTP(w, r)
			return
		}

		gzw, err := gzip.NewWriterLevel(w, gzip.BestSpeed)
		if err != nil {
			io.WriteString(w, err.Error())
			return
		}
		defer gzw.Close()

		next.ServeHTTP(&gzipResponseWriter{ResponseWriter: w, Writer: gzw}, r)
	})
}

func clientAcceptsGzip(r *http.Request) bool {
	ae := r.Header.Get("Accept-Encoding")
	return strings.Contains(ae, "gzip")
}

func shouldDecompressRequest(r *http.Request) bool {
	ce := r.Header.Get("Content-Encoding")
	if !strings.Contains(ce, "gzip") {
		return false
	}

	ct := r.Header.Get("Content-Type")
	return strings.HasPrefix(ct, "application/json") || strings.HasPrefix(ct, "text/html")
}
