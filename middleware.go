package otintegration

import (
	"github.com/ant0ine/go-json-rest/rest"
	"github.com/gin-gonic/gin"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
)

// OpenTracerGinMiddleware - middleware that adds opentracing
func OpenTracerGinMiddleware(operationPrefix []byte, tracer opentracing.Tracer) gin.HandlerFunc {
	if operationPrefix == nil {
		operationPrefix = []byte("api-request")
	}
	return func(c *gin.Context) {
		// all before request is handled
		var span opentracing.Span
		var err error
		if cspan, ok := c.Get(spanContextKey); ok {
			span = StartSpanWithParent(tracer, cspan.(opentracing.Span).Context(), string(operationPrefix)+" "+c.Request.URL.Path, c.Request.Method, c.Request.URL.Path)

		} else {
			span, err = StartSpanWithHeader(tracer, &c.Request.Header, string(operationPrefix)+" "+c.Request.URL.Path, c.Request.Method, c.Request.URL.Path)
			if err != nil {
				c.Next()
				return
			}
		}
		defer span.Finish()
		c.Set(spanContextKey, span)
		c.Next()

		span.SetTag(string(ext.HTTPStatusCode), c.Writer.Status())
	}
}

// OpenTracerGorestMiddleware - middleware that adds opentracing
func OpenTracerGorestMiddleware(operationPrefix []byte, tracer opentracing.Tracer) rest.MiddlewareSimple {
	return func(next rest.HandlerFunc) rest.HandlerFunc {
		if operationPrefix == nil {
			operationPrefix = []byte("api-request")
		}
		return func(w rest.ResponseWriter, r *rest.Request) {
			var span opentracing.Span
			var err error
			if cspan, ok := r.Env[spanContextKey]; ok {
				span = StartSpanWithParent(tracer, cspan.(opentracing.Span).Context(), string(operationPrefix)+" "+r.URL.Path, r.Method, r.URL.Path)

			} else {
				span, err = StartSpanWithHeader(tracer, &r.Header, string(operationPrefix)+" "+r.URL.Path, r.Method, r.URL.Path)
				if err != nil {
					next(w, r)
					return
				}
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
