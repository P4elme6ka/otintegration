package otintegration

import (
	"bytes"
	"errors"
	"github.com/ant0ine/go-json-rest/rest"
	"github.com/gin-gonic/gin"
	"github.com/opentracing/opentracing-go/log"
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
	GetBuff() *bytes.Buffer
}

// StartSpan will start a new span with no parent span.
func StartSpan(tracer opentracing.Tracer, operationName, method, path string) opentracing.Span {
	return StartSpanWithParent(tracer, nil, operationName, method, path)
}

// StartSpanWithParent will start a new span with a parent span.
func StartSpanWithParent(tracer opentracing.Tracer, parent opentracing.SpanContext, operationName, method, path string) opentracing.Span {
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
func StartSpanWithHeader(tracer opentracing.Tracer, header *http.Header, operationName, method, path string) (opentracing.Span, error) {
	if header != nil {
		ctx, err := tracer.Extract(opentracing.HTTPHeaders, opentracing.HTTPHeadersCarrier(*header))
		if err != nil {
			return nil, err
		}
		return StartSpanWithParent(tracer, ctx, operationName, method, path), nil
	}
	return StartSpanWithParent(tracer, nil, operationName, method, path), nil
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

// GetGinSubSpan return new span from gin context
func GetGinSubSpan(ctx *gin.Context, operationName string) (opentracing.Span, error) {
	s, err := GetGinSpan(ctx)
	if err != nil {
		return nil, err
	}
	span := GetSubSpan(s, operationName)
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

// GetGorestSubSpan extracts span and returns new span from gorest request.
func GetGorestSubSpan(r *rest.Request, operationName string) (opentracing.Span, error) {
	s, err := GetGorestSpan(r)
	if err != nil {
		return nil, err
	}
	sub := GetSubSpan(s, operationName)
	return sub, nil
}

// ExtractFromBinary extracts context from Injectable interface
func ExtractFromBinary(tracer opentracing.Tracer, inter Injectable) (opentracing.SpanContext, error) {
	var bt []byte
	_, err := inter.GetBuff().Read(bt)
	if err != nil {
		return nil, err
	}
	inter.GetBuff().Write(bt)
	bf := bytes.NewBuffer(bt)

	spanCtx, err := tracer.Extract(opentracing.Binary, bf)
	return spanCtx, err
}

func InjectToBinary(r *rest.Request, inter Injectable) error {
	span, err := GetGorestSpan(r)
	if err != nil {
		return err
	}
	tracer := span.Tracer()
	err = tracer.Inject(span.Context(), opentracing.Binary, inter.GetBuff())
	return err
}

// StartSpanFromBinary return new span from Injectable interface
func StartSpanFromBinary(tracer opentracing.Tracer, inter Injectable, operName string) (opentracing.Span, error) {
	ctx, err := ExtractFromBinary(tracer, inter)
	return StartSpanWithBinParent(tracer, ctx, operName), err
}

// GetSubSpan return new span from existing
func GetSubSpan(spanRoot opentracing.Span, operationName string, opt ...opentracing.StartSpanOption) opentracing.Span {
	opt = append(opt, opentracing.ChildOf(spanRoot.Context()))
	return spanRoot.Tracer().StartSpan(operationName, opt...)
}

func NewEmptySpan() opentracing.Span {
	return EmptySpan{}
}

type EmptySpan struct{}

func (e EmptySpan) Finish() {}

func (e EmptySpan) FinishWithOptions(opentracing.FinishOptions) {}

func (e EmptySpan) Context() opentracing.SpanContext { return nil }

func (e EmptySpan) SetOperationName(string) opentracing.Span { return nil }

func (e EmptySpan) SetTag(string, interface{}) opentracing.Span { return nil }

func (e EmptySpan) LogFields(...log.Field) {}

func (e EmptySpan) LogKV(...interface{}) {}

func (e EmptySpan) SetBaggageItem(string) opentracing.Span { return nil }

func (e EmptySpan) BaggageItem(string) string { return "" }

func (e EmptySpan) Tracer() opentracing.Tracer { return nil }

func (e EmptySpan) LogEvent(string) {}

func (e EmptySpan) LogEventWithPayload(interface{}) {}

func (e EmptySpan) Log(opentracing.LogData) {}
