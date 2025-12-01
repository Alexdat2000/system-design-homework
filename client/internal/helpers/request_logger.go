package helpers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog"
)

var logger zerolog.Logger

func init() {
	logger = zerolog.New(os.Stdout).
		With().
		Timestamp().
		Logger().
		Level(zerolog.InfoLevel)
}

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

func extractBusinessMetrics(body []byte) map[string]interface{} {
	metrics := make(map[string]interface{})

	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return metrics
	}

	if userID, ok := data["user_id"].(string); ok && userID != "" {
		metrics["user_id"] = userID
	}
	if orderID, ok := data["order_id"].(string); ok && orderID != "" {
		metrics["order_id"] = orderID
	}
	if offerID, ok := data["offer_id"].(string); ok && offerID != "" {
		metrics["offer_id"] = offerID
	}
	if scooterID, ok := data["scooter_id"].(string); ok && scooterID != "" {
		metrics["scooter_id"] = scooterID
	}

	return metrics
}

func extractResponseMetrics(body []byte) map[string]interface{} {
	metrics := make(map[string]interface{})

	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return metrics
	}

	if id, ok := data["id"].(string); ok && id != "" {
		if orderID, ok := data["order_id"].(string); ok && orderID != "" {
			metrics["order_id"] = orderID
		} else {
			if _, isOffer := data["expires_at"]; isOffer {
				metrics["offer_id"] = id
			} else {
				metrics["order_id"] = id
			}
		}
	}
	if userID, ok := data["user_id"].(string); ok && userID != "" {
		metrics["user_id"] = userID
	}
	if status, ok := data["status"].(string); ok && status != "" {
		metrics["order_status"] = status
	}

	return metrics
}

func RequestLoggerWithBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		requestID := middleware.GetReqID(r.Context())

		logEvent := logger.Info().
			Str("event", "request_start").
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Str("remote_addr", r.RemoteAddr).
			Str("user_agent", r.UserAgent())

		if requestID != "" {
			logEvent = logEvent.Str("request_id", requestID)
		}
		if r.URL.RawQuery != "" {
			logEvent = logEvent.Str("query", r.URL.RawQuery)
		}

		if r.URL.Path != "" {
			if orderID := chi.URLParam(r, "order_id"); orderID != "" {
				logEvent = logEvent.Str("order_id", orderID)
			}
		}

		var requestMetrics map[string]interface{}
		if r.Body != nil {
			bodyBytes, err := io.ReadAll(r.Body)
			if err == nil && len(bodyBytes) > 0 {
				requestMetrics = extractBusinessMetrics(bodyBytes)
				r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

				for k, v := range requestMetrics {
					if strVal, ok := v.(string); ok {
						logEvent = logEvent.Str(k, strVal)
					}
				}

				const maxRequestBody = 1000
				requestBodyStr := string(bodyBytes)
				if len(requestBodyStr) > maxRequestBody {
					requestBodyStr = requestBodyStr[:maxRequestBody] + "...(truncated)"
				}
				if requestBodyStr != "" {
					logEvent = logEvent.Str("request_body", requestBodyStr)
				}
			}
		}

		logEvent.Msg("HTTP request started")

		lrw := &loggingResponseWriter{ResponseWriter: w}

		next.ServeHTTP(lrw, r)

		duration := time.Since(start)
		durationMs := float64(duration.Nanoseconds()) / 1e6

		responseMetrics := extractResponseMetrics(lrw.body.Bytes())

		logEvent = logger.Info().
			Str("event", "request_complete").
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Int("status_code", lrw.statusCode).
			Float64("duration_ms", durationMs).
			Dur("duration", duration).
			Str("remote_addr", r.RemoteAddr)

		if requestID != "" {
			logEvent = logEvent.Str("request_id", requestID)
		}

		for k, v := range requestMetrics {
			if strVal, ok := v.(string); ok {
				logEvent = logEvent.Str(k, strVal)
			}
		}

		for k, v := range responseMetrics {
			if strVal, ok := v.(string); ok {
				logEvent = logEvent.Str(k, strVal)
			}
		}

		if lrw.statusCode >= 400 {
			logEvent = logEvent.
				Str("level", "error").
				Int("error_code", lrw.statusCode)

			const maxErrorBody = 500
			respBody := lrw.body.String()
			if len(respBody) > maxErrorBody {
				respBody = respBody[:maxErrorBody] + "...(truncated)"
			}
			if respBody != "" {
				logEvent = logEvent.Str("error_response", respBody)
			}
		}

		if lrw.statusCode >= 500 {
			logEvent.Msg("HTTP request failed")
		} else if lrw.statusCode >= 400 {
			logEvent.Msg("HTTP request client error")
		} else {
			logEvent.Msg("HTTP request completed")
		}
	})
}
