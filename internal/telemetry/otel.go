package telemetry

import (
	"context"
	"fmt"
	"log/slog"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// InitTracer initializes an OpenTelemetry tracer provider that prints spans to stdout.
func InitTracer() (func(context.Context) error, error) {
	exporter, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout exporter: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		// In production, you would add WithResource() here
	)
	otel.SetTracerProvider(tp)
	slog.Info("OpenTelemetry initialized with stdout exporter")

	return tp.Shutdown, nil
}
