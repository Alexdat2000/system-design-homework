package helpers

import (
	"bytes"
	"log"
	"net/http"
	"time"
)

type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
	body       bytes.Buffer
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

func (lrw *loggingResponseWriter) Write(b []byte) (int, error) {
	if lrw.statusCode == 0 {
		lrw.statusCode = http.StatusOK
	}
	lrw.body.Write(b)
	return lrw.ResponseWriter.Write(b)
}

func RequestLoggerWithBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		lrw := &loggingResponseWriter{ResponseWriter: w}

		next.ServeHTTP(lrw, r)

		duration := time.Since(start)

		const maxLogBody = 2000
		respBody := lrw.body.String()
		if len(respBody) > maxLogBody {
			respBody = respBody[:maxLogBody] + "...(truncated)"
		}

		log.Printf(
			"%s %s status=%d duration=%s remote=%s query=%q response=%s",
			r.Method,
			r.URL.Path,
			lrw.statusCode,
			duration,
			r.RemoteAddr,
			r.URL.RawQuery,
			respBody,
		)
	})
}
