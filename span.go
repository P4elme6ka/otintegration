package otracing2gin

import (
	"errors"
	"github.com/ant0ine/go-json-rest/rest"
	"github.com/gin-gonic/gin"
	"net/http"
	"runtime"

	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
)

const spanContextKey = "tracing-context"

// ErrSpanNotFound Errors which may occur at operation time.
var (
	ErrSpanNotFound = errors.New("span was not found in context")
)

// StartSpan will start a new span with no parent span.
func StartSpan(operationName, method, path string) opentracing.Span {
	return StartSpanWithParent(nil, operationName, method, path)
}

// StartDBSpanWithParent - start a DB operation span
func StartDBSpanWithParent(parent opentracing.SpanContext, operationName, dbInstance, dbType, dbStatement string) opentracing.Span {
	options := []opentracing.StartSpanOption{opentracing.Tag{Key: ext.SpanKindRPCServer.Key, Value: ext.SpanKindRPCServer.Value}}
	if len(dbInstance) > 0 {
		options = append(options, opentracing.Tag{Key: string(ext.DBInstance), Value: dbInstance})
	}
	if len(dbType) > 0 {
		options = append(options, opentracing.Tag{Key: string(ext.DBType), Value: dbType})
	}
	if len(dbStatement) > 0 {
		options = append(options, opentracing.Tag{Key: string(ext.DBStatement), Value: dbStatement})
	}
	if parent != nil {
		options = append(options, opentracing.ChildOf(parent))
	}

	return opentracing.StartSpan(operationName, options...)
}

// StartSpanWithParent will start a new span with a parent span.
// example:
//      span:= StartSpanWithParent(c.Get(spanContextKey),
func StartSpanWithParent(parent opentracing.SpanContext, operationName, method, path string) opentracing.Span {
	options := []opentracing.StartSpanOption{
		opentracing.Tag{Key: ext.SpanKindRPCServer.Key, Value: ext.SpanKindRPCServer.Value},
		opentracing.Tag{Key: string(ext.HTTPMethod), Value: method},
		opentracing.Tag{Key: string(ext.HTTPUrl), Value: path},
		opentracing.Tag{Key: "current-goroutines", Value: runtime.NumGoroutine()},
	}

	if parent != nil {
		options = append(options, opentracing.ChildOf(parent))
	}

	return opentracing.StartSpan(operationName, options...)
}

// StartSpanWithHeader will look in the headers to look for a parent span before starting the new span.
// example:
//  func handleGet(c *gin.Context) {
//     span := StartSpanWithHeader(&c.Request.Header, "api-request", method, path)
//     defer span.Finish()
//     c.Set(spanContextKey, span) // add the span to the context so it can be used for the duration of the request.
//     bosePersonID := c.Param("bosePersonID")
//     span.SetTag("bosePersonID", bosePersonID)
//
func StartSpanWithHeader(header *http.Header, operationName, method, path string) opentracing.Span {
	var wireContext opentracing.SpanContext
	if header != nil {
		wireContext, _ = opentracing.GlobalTracer().Extract(opentracing.HTTPHeaders, opentracing.HTTPHeadersCarrier(*header))
	}
	return StartSpanWithParent(wireContext, operationName, method, path)
}

// InjectTraceID injects the span ID into the provided HTTP header object, so that the
// current span will be propogated downstream to the server responding to an HTTP request.
// Specifying the span ID in this way will allow the tracing system to connect spans
// between servers.
//
//  Usage:
//          // resty example
// 	    r := resty.R()
//	    injectTraceID(span, r.Header)
//	    resp, err := r.Get(fmt.Sprintf("http://localhost:8000/users/%s", bosePersonID))
//
//          // galapagos_clients example
//          c := galapagos_clients.GetHTTPClient()
//          req, err := http.NewRequest("GET", fmt.Sprintf("http://localhost:8000/users/%s", bosePersonID))
//          injectTraceID(span, req.Header)
//          c.Do(req)
func InjectTraceID(ctx opentracing.SpanContext, header http.Header) {
	opentracing.GlobalTracer().Inject(
		ctx,
		opentracing.HTTPHeaders,
		opentracing.HTTPHeadersCarrier(header))
}

// NewSpan returns gin.HandlerFunc (middleware) that starts a new span and injects it to request context.
//
// It calls ctx.Next() to measure execution time of all following handlers.
func NewSpan(tracer opentracing.Tracer, operationName string, opts ...opentracing.StartSpanOption) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		span := tracer.StartSpan(operationName, opts...)
		ctx.Set(spanContextKey, span)
		defer span.Finish()

		ctx.Next()
	}
}

// ParentSpanReferenceFunc determines how to reference parent span
//
// See opentracing.SpanReferenceType
type ParentSpanReferenceFunc func(opentracing.SpanContext) opentracing.StartSpanOption

// SpanFromHeaders returns gin.HandlerFunc (middleware) that extracts parent span data from HTTP headers in TextMap format and
// starts a new span referenced to parent with ParentSpanReferenceFunc.
//
// It calls ctx.Next() to measure execution time of all following handlers.
//
// Behaviour on errors determined by abortOnErrors option. If it set to true request handling will be aborted with error.
func SpanFromHeaders(tracer opentracing.Tracer, operationName string, psr ParentSpanReferenceFunc,
	abortOnErrors bool, advancedOpts ...opentracing.StartSpanOption) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		spanContext, err := tracer.Extract(opentracing.TextMap, opentracing.HTTPHeadersCarrier(ctx.Request.Header))
		if err != nil {
			if abortOnErrors {
				ctx.AbortWithError(http.StatusInternalServerError, err)
			}
			return
		}

		opts := append([]opentracing.StartSpanOption{psr(spanContext)}, advancedOpts...)

		span := tracer.StartSpan(operationName, opts...)
		ctx.Set(spanContextKey, span)
		defer span.Finish()

		ctx.Next()
	}
}

// SpanFromHeadersHTTPFmt returns gin.HandlerFunc (middleware) that extracts parent span data from HTTP headers in HTTPHeaders format and
// starts a new span referenced to parent with ParentSpanReferenceFunc.
//
// It calls ctx.Next() to measure execution time of all following handlers.
//
// Behaviour on errors determined by abortOnErrors option. If it set to true request handling will be aborted with error.
func SpanFromHeadersHTTPFmt(tracer opentracing.Tracer, operationName string, psr ParentSpanReferenceFunc,
	abortOnErrors bool, advancedOpts ...opentracing.StartSpanOption) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		spanContext, err := tracer.Extract(opentracing.HTTPHeaders, opentracing.HTTPHeadersCarrier(ctx.Request.Header))
		if err != nil {
			if abortOnErrors {
				ctx.AbortWithError(http.StatusInternalServerError, err)
			}
			return
		}

		opts := append([]opentracing.StartSpanOption{psr(spanContext)}, advancedOpts...)

		span := tracer.StartSpan(operationName, opts...)
		ctx.Set(spanContextKey, span)
		defer span.Finish()

		ctx.Next()
	}
}

// SpanFromContext returns gin.HandlerFunc (middleware) that extracts parent span from request context
// and starts a new span as child of parent span.
//
// It calls ctx.Next() to measure execution time of all following handlers.
//
// Behaviour on errors determined by abortOnErrors option. If it set to true request handling will be aborted with error.
func SpanFromContext(tracer opentracing.Tracer, operationName string, abortOnErrors bool,
	advancedOpts ...opentracing.StartSpanOption) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		var opts []opentracing.StartSpanOption
		parentSpanI, _ := ctx.Get(spanContextKey)
		if parentSpan, typeOk := parentSpanI.(opentracing.Span); parentSpan != nil && typeOk {
			opts = append(opts, opentracing.ChildOf(parentSpan.Context()))
		} else {
			if abortOnErrors {
				ctx.AbortWithError(http.StatusInternalServerError, ErrSpanNotFound)
			}
			return
		}
		opts = append(opts, advancedOpts...)

		span := tracer.StartSpan(operationName, opts...)
		ctx.Set(spanContextKey, span)
		defer span.Finish()

		ctx.Next()
	}
}

// InjectToHeaders injects span meta-information to request headers.
//
// It may be useful when you want to trace chained request (client->service 1->service 2).
// In this case you have to save request headers (ctx.Request.Header) and pass it to next level request.
//
// Behaviour on errors determined by abortOnErrors option. If it set to true request handling will be aborted with error.
func InjectToHeaders(tracer opentracing.Tracer, abortOnErrors bool) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		var spanContext opentracing.SpanContext
		spanI, _ := ctx.Get(spanContextKey)
		if span, typeOk := spanI.(opentracing.Span); span != nil && typeOk {
			spanContext = span.Context()
		} else {
			if abortOnErrors {
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, ErrSpanNotFound)
			}
			return
		}

		tracer.Inject(spanContext, opentracing.HTTPHeaders, opentracing.HTTPHeadersCarrier(ctx.Request.Header))
	}
}

// GetGinSpan extracts span from gin context.
func GetGinSpan(ctx *gin.Context) (opentracing.Span, error) {
	spanI, _ := ctx.Get(spanContextKey)
	span, ok := spanI.(opentracing.Span)
	if span == nil || !ok {
		return nil, ErrSpanNotFound
	}
	return span, nil
}

// GetGorestSpan extracts span from gorest request.
func GetGorestSpan(r *rest.Request) (opentracing.Span, error) {
	spanI, _ := r.Env[spanContextKey]
	span, ok := spanI.(opentracing.Span)
	if span == nil || !ok {
		return nil, ErrSpanNotFound
	}
	return span, nil
}

// GetGorestSubSpan extracts span from gorest request.
func GetGorestSubSpan(r *rest.Request, operationName string) (opentracing.Span, error) {
	spanI, _ := r.Env[spanContextKey]
	span, ok := spanI.(opentracing.Span)
	if span == nil || !ok {
		return nil, ErrSpanNotFound
	}
	sub := GetSubSpan(span, operationName)
	return sub, nil
}

func GetSubSpan(spanRoot opentracing.Span, operationName string, opt ...opentracing.StartSpanOption) opentracing.Span {
	opt = append(opt, opentracing.ChildOf(spanRoot.Context()))
	return spanRoot.Tracer().StartSpan(operationName, opt...)
}
