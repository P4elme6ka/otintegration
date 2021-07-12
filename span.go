package otintegration

import (
	"bytes"
	"errors"
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

type TraceInfo interface {
	io.ReadWriter
	Reset()
	Check() bool
}

// StartSpan will start a new span with no parent span.
func StartSpan(tracer opentracing.Tracer, operationName, method, path string) opentracing.Span {
	return StartSpanWithParent(tracer, nil, operationName, method, path)
}

// StartSpanWithParent will start a new span with a parent span.
func StartSpanWithParent(tracer opentracing.Tracer, parent opentracing.SpanContext, operationName, method, path string) opentracing.Span {
	options := []opentracing.StartSpanOption{
		opentracing.Tag{Key: string(ext.HTTPMethod), Value: method},
		opentracing.Tag{Key: string(ext.HTTPUrl), Value: path},
		opentracing.Tag{Key: ext.SpanKindRPCServer.Key, Value: ext.SpanKindRPCServer.Value},
		opentracing.Tag{Key: "current-goroutines", Value: runtime.NumGoroutine()},
	}

	bytes.NewBuffer([]byte("ff"))
	if parent != nil {
		options = append(options, opentracing.ChildOf(parent))
	}
	return tracer.StartSpan(operationName, options...)
}

func StartSpanWithBinParent(tracer opentracing.Tracer, parent opentracing.SpanContext, operationName string) opentracing.Span {
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
func StartSpanWithHeader(tracer opentracing.Tracer, header *http.Header, operationName, method, path string) opentracing.Span {
	if header != nil {
		ctx, err := tracer.Extract(opentracing.HTTPHeaders, opentracing.HTTPHeadersCarrier(*header))
		if err != nil {
			StartSpanWithParent(tracer, nil, operationName, method, path)
		}
		return StartSpanWithParent(tracer, ctx, operationName, method, path)
	}
	return StartSpanWithParent(tracer, nil, operationName, method, path)
}

// ExtractFromBinary extracts context from Injectable interface
func ExtractFromBinary(tracer opentracing.Tracer, inter TraceInfo) (opentracing.SpanContext, error) {
	spanCtx, err := tracer.Extract(opentracing.Binary, inter)
	if err != nil {
		return nil, err
	}
	return spanCtx, nil
}

// InjectToBinary resets TracerInfo buff, and inject context to interface
func InjectToBinary(tracer opentracing.Tracer, ctx opentracing.SpanContext, inter TraceInfo) {
	inter.Reset()
	tracer.Inject(ctx, opentracing.Binary, inter)
}

// StartSpanFromBinary return new span from Injectable interface
func StartSpanFromBinary(tracer opentracing.Tracer, inter TraceInfo, operName string) (opentracing.Span, error) {
	ctx, err := ExtractFromBinary(tracer, inter)
	return StartSpanWithBinParent(tracer, ctx, operName), err
}

// GetSubSpan return new span from existing
func GetSubSpan(spanRoot opentracing.Span, operationName string) opentracing.Span {
	options := []opentracing.StartSpanOption{
		opentracing.ChildOf(spanRoot.Context()),
		opentracing.Tag{Key: ext.SpanKindRPCServer.Key, Value: ext.SpanKindRPCServer.Value},
		opentracing.Tag{Key: "current-goroutines", Value: runtime.NumGoroutine()},
	}
	return spanRoot.Tracer().StartSpan(operationName, options...)
}

func NewEmptySpan() opentracing.Span {
	return opentracing.NoopTracer{}.StartSpan("")
}
