// Package tracing provides OpenTelemetry middleware for the Chronos API.
package tracing

import (
	"fmt"
	"net/http"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// Middleware returns an HTTP middleware that creates spans for every request
// and injects the trace ID into the response via X-Trace-ID.
func Middleware(serviceName string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract any incoming trace context
			ctx := ExtractTraceContext(r.Context(), propagationCarrier(r.Header))

			ctx, span := tracer.Start(ctx, fmt.Sprintf("%s %s", r.Method, r.URL.Path),
				trace.WithSpanKind(trace.SpanKindServer),
				trace.WithAttributes(
					attribute.String("http.method", r.Method),
					attribute.String("http.url", r.URL.String()),
					attribute.String("http.target", r.URL.Path),
					attribute.String("http.host", r.Host),
					attribute.String("http.user_agent", r.UserAgent()),
				),
			)
			defer span.End()

			// Set trace ID in response header for correlation
			sc := span.SpanContext()
			if sc.HasTraceID() {
				w.Header().Set("X-Trace-ID", sc.TraceID().String())
			}

			// Wrap writer to capture status code
			sw := &statusWriter{ResponseWriter: w, statusCode: http.StatusOK}
			next.ServeHTTP(sw, r.WithContext(ctx))

			span.SetAttributes(attribute.Int("http.status_code", sw.statusCode))
			if sw.statusCode >= 400 {
				span.SetAttributes(attribute.Bool("error", true))
			}
		})
	}
}

// propagationCarrier adapts http.Header to TextMapCarrier.
type propagationCarrier http.Header

func (c propagationCarrier) Get(key string) string {
	return http.Header(c).Get(key)
}
func (c propagationCarrier) Set(key, value string) {
	http.Header(c).Set(key, value)
}
func (c propagationCarrier) Keys() []string {
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}
	return keys
}

// statusWriter wraps http.ResponseWriter to capture the status code.
type statusWriter struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

func (w *statusWriter) WriteHeader(code int) {
	if !w.written {
		w.statusCode = code
		w.written = true
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if !w.written {
		w.written = true
	}
	return w.ResponseWriter.Write(b)
}
