package otintegration

import (
	"github.com/ant0ine/go-json-rest/rest"
	"github.com/gin-gonic/gin"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
)

// OpenTracerGinMiddleware - middleware that adds opentracing
func OpenTracerGinMiddleware(operationPrefix string, tracer opentracing.Tracer) gin.HandlerFunc {
	return func(c *gin.Context) {
		// all before request is handled
		var span opentracing.Span
		if cspan, ok := c.Get(spanContextKey); ok {
			span = StartSpanWithParent(tracer, cspan.(opentracing.Span).Context(), operationPrefix+" "+c.Request.URL.Path, c.Request.Method, c.Request.URL.Path)

		} else {
			span = StartSpanWithHeader(tracer, &c.Request.Header, operationPrefix+" "+c.Request.URL.Path, c.Request.Method, c.Request.URL.Path)
		}
		defer span.Finish()
		c.Set(spanContextKey, span)
		c.Next()

		span.SetTag(string(ext.HTTPStatusCode), c.Writer.Status())
	}
}

// OpenTracerGorestMiddleware - middleware that adds opentracing
func OpenTracerGorestMiddleware(operationPrefix string, tracer opentracing.Tracer) rest.MiddlewareSimple {
	return func(next rest.HandlerFunc) rest.HandlerFunc {
		return func(w rest.ResponseWriter, r *rest.Request) {
			var span opentracing.Span
			if cspan, ok := r.Env[spanContextKey]; ok {
				span = StartSpanWithParent(tracer, cspan.(opentracing.Span).Context(), operationPrefix+" "+r.URL.Path, r.Method, r.URL.Path)

			} else {
				span = StartSpanWithHeader(tracer, &r.Header, operationPrefix+" "+r.URL.Path, r.Method, r.URL.Path)
			}
			defer span.Finish() // after all the other defers are completed, finish the span

			r.Env[spanContextKey] = span

			next(w, r)

			status, ok := r.Env["STATUS_CODE"].(int)
			if ok {
				span.SetTag(string(ext.HTTPStatusCode), status)
			}
		}
	}
}
