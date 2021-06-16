package otracing2gin

import (
	"errors"
	"github.com/ant0ine/go-json-rest/rest"
	"github.com/gin-gonic/gin"
	"github.com/opentracing/opentracing-go/log"
	"io"
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

type Injectable interface {
	io.Writer
	io.Reader
}

// StartSpan will start a new span with no parent span.
func StartSpan(operationName, method, path string, tracer opentracing.Tracer) opentracing.Span {
	return StartSpanWithParent(nil, operationName, method, path, tracer)
}

// StartDBSpanWithParent - start a DB operation span
func StartDBSpanWithParent(parent opentracing.SpanContext, operationName, dbInstance, dbType, dbStatement string, tracer opentracing.Tracer) opentracing.Span {
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

	return tracer.StartSpan(operationName, options...)
}

// StartSpanWithParent will start a new span with a parent span.
func StartSpanWithParent(parent opentracing.SpanContext, operationName, method, path string, tracer opentracing.Tracer) opentracing.Span {
	options := []opentracing.StartSpanOption{
		opentracing.Tag{Key: ext.SpanKindRPCServer.Key, Value: ext.SpanKindRPCServer.Value},
		opentracing.Tag{Key: string(ext.HTTPMethod), Value: method},
		opentracing.Tag{Key: string(ext.HTTPUrl), Value: path},
		opentracing.Tag{Key: "current-goroutines", Value: runtime.NumGoroutine()},
	}

	if parent != nil {
		options = append(options, opentracing.ChildOf(parent))
	}
	return tracer.StartSpan(operationName, options...)
}

func StartSpanWithBinParent(parent opentracing.SpanContext, operationName string, tracer opentracing.Tracer) opentracing.Span {
	options := []opentracing.StartSpanOption{
		opentracing.Tag{Key: ext.SpanKindRPCServer.Key, Value: ext.SpanKindRPCServer.Value},
		opentracing.Tag{Key: "current-goroutines", Value: runtime.NumGoroutine()},
	}

	if parent != nil {
		options = append(options, opentracing.ChildOf(parent))
	}

	return tracer.StartSpan(operationName, options...)
}

// StartSpanWithHeader will look in the headers to look for a parent span before starting the new span.
func StartSpanWithHeader(header *http.Header, tracer opentracing.Tracer, operationName, method, path string) opentracing.Span {
	var wireContext opentracing.SpanContext
	if header != nil {
		wireContext, _ = tracer.Extract(opentracing.HTTPHeaders, opentracing.HTTPHeadersCarrier(*header))
	}
	return StartSpanWithParent(wireContext, operationName, method, path, tracer)
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

func InjectToBinary(r *rest.Request, inter Injectable) {
	span, _ := GetGorestSpan(r)
	tracer := span.Tracer()
	_ = tracer.Inject(span.Context(), opentracing.Binary, inter) // TODO: error handling
}

func ExtractFromBinary(tracer opentracing.Tracer, inter Injectable) (opentracing.SpanContext, error) {
	spanCtx, err := tracer.Extract(opentracing.Binary, inter) // TODO: error handling
	return spanCtx, err
}

func StartSpanFromBinary(tracer opentracing.Tracer, inter Injectable, operName string) (opentracing.Span, error) {
	ctx, err := ExtractFromBinary(tracer, inter)
	return StartSpanWithBinParent(ctx, operName, tracer), err
}

func GetSubSpan(spanRoot opentracing.Span, operationName string, opt ...opentracing.StartSpanOption) opentracing.Span {
	opt = append(opt, opentracing.ChildOf(spanRoot.Context()))
	return spanRoot.Tracer().StartSpan(operationName, opt...)
}

func NewEmptySpan() opentracing.Span {
	return EmptySpan{}
}

type EmptySpan struct{}

func (e EmptySpan) Finish() {}

func (e EmptySpan) FinishWithOptions(opts opentracing.FinishOptions) {}

func (e EmptySpan) Context() opentracing.SpanContext { return nil }

func (e EmptySpan) SetOperationName(operationName string) opentracing.Span { return nil }

func (e EmptySpan) SetTag(key string, value interface{}) opentracing.Span { return nil }

func (e EmptySpan) LogFields(fields ...log.Field) {}

func (e EmptySpan) LogKV(alternatingKeyValues ...interface{}) {}

func (e EmptySpan) SetBaggageItem(restrictedKey, value string) opentracing.Span { return nil }

func (e EmptySpan) BaggageItem(restrictedKey string) string { return "" }

func (e EmptySpan) Tracer() opentracing.Tracer { return nil }

func (e EmptySpan) LogEvent(event string) {}

func (e EmptySpan) LogEventWithPayload(event string, payload interface{}) {}

func (e EmptySpan) Log(data opentracing.LogData) {}
