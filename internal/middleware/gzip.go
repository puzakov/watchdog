package middleware

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"sync"
)

var gzipWriterPool = sync.Pool{
	New: func() any {
		w, _ := gzip.NewWriterLevel(io.Discard, gzip.BestSpeed)
		return w
	},
}

type gzipResponseWriter struct {
	http.ResponseWriter
	gzw      *gzip.Writer
	compress bool
	isInit   bool
}

func (w *gzipResponseWriter) init() {
	if w.isInit {
		return
	}

	ct := w.Header().Get("Content-Type")
	if strings.HasPrefix(ct, "application/json") || strings.HasPrefix(ct, "text/html") {
		gzw := gzipWriterPool.Get().(*gzip.Writer)
		gzw.Reset(w.ResponseWriter)
		w.gzw = gzw
		w.compress = true
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Add("Vary", "Accept-Encoding")
	}

	w.isInit = true
}

func (w *gzipResponseWriter) WriteHeader(statusCode int) {
	w.init()
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *gzipResponseWriter) Write(b []byte) (int, error) {
	w.init()

	if w.compress {
		return w.gzw.Write(b)
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

		grw := &gzipResponseWriter{ResponseWriter: w}
		defer func() {
			if grw.gzw != nil {
				_ = grw.gzw.Close()
				gzipWriterPool.Put(grw.gzw)
				grw.gzw = nil
			}
		}()

		next.ServeHTTP(grw, r)
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
