package otintegration

import (
	"github.com/ant0ine/go-json-rest/rest"
	"github.com/opentracing/opentracing-go"
)

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

func InjectGorestToBinary(r *rest.Request, inter TraceInfo) error {
	span, err := GetGorestSpan(r)
	if err != nil {
		return err
	}
	tracer := span.Tracer()
	err = tracer.Inject(span.Context(), opentracing.Binary, inter)
	return err
}
