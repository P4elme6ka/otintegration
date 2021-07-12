package otintegration

import (
	"github.com/gin-gonic/gin"
	"github.com/opentracing/opentracing-go"
)

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

func InjectGinToBinary(c *gin.Context, inter TraceInfo) error {
	span, err := GetGinSpan(c)
	if err != nil {
		return err
	}
	tracer := span.Tracer()
	err = tracer.Inject(span.Context(), opentracing.Binary, inter)
	return err
}
