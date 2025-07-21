package semconvprocessor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.uber.org/zap"

	"github.com/cedricziel/semconvprocessor/processors/semconvprocessor/internal/metadata"
)

func TestProcessTracesRespectsExistingAttributes(t *testing.T) {
	ctx := context.Background()
	
	cfg := &Config{
		Enabled: true,
		SpanProcessing: SpanProcessingConfig{
			Enabled:                true,
			Mode:                   ModeEnforce,
			OperationNameAttribute: "operation.name",
			OperationTypeAttribute: "operation.type",
			Rules: []OTTLRule{
				{
					ID:            "http_rule",
					Priority:      100,
					Condition:     `attributes["http.method"] != nil`,
					OperationName: `Concat([attributes["http.method"], "/test"], " ")`,
					OperationType: `"http"`,
				},
			},
		},
	}
	
	// Build processor
	telemetrySettings := processortest.NewNopSettings(component.MustNewType("semconv")).TelemetrySettings
	telemetryBuilder, _ := metadata.NewTelemetryBuilder(telemetrySettings)
	sp, err := newSemconvProcessor(zap.NewNop(), cfg, telemetryBuilder, telemetrySettings)
	require.NoError(t, err)
	
	// Test data with pre-existing operation.name
	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()
	
	// Span 1: Has pre-existing operation.name - should be skipped
	span1 := ss.Spans().AppendEmpty()
	span1.SetName("original_span_1")
	span1.Attributes().PutStr("http.method", "GET")
	span1.Attributes().PutStr("operation.name", "pre-existing-operation")
	span1.Attributes().PutStr("operation.type", "pre-existing-type")
	
	// Span 2: Has no operation.name - should be processed
	span2 := ss.Spans().AppendEmpty()
	span2.SetName("original_span_2")
	span2.Attributes().PutStr("http.method", "POST")
	
	// Span 3: Has operation.type but no operation.name - should be processed but type preserved
	span3 := ss.Spans().AppendEmpty()
	span3.SetName("original_span_3")
	span3.Attributes().PutStr("http.method", "PUT")
	span3.Attributes().PutStr("operation.type", "pre-existing-type")
	
	// Process
	result, err := sp.processTraces(ctx, traces)
	require.NoError(t, err)
	
	// Verify results
	resultSpans := result.ResourceSpans().At(0).ScopeSpans().At(0).Spans()
	
	// Span 1: Should remain unchanged
	assert.Equal(t, "original_span_1", resultSpans.At(0).Name())
	operationName, _ := resultSpans.At(0).Attributes().Get("operation.name")
	assert.Equal(t, "pre-existing-operation", operationName.Str())
	operationType, _ := resultSpans.At(0).Attributes().Get("operation.type")
	assert.Equal(t, "pre-existing-type", operationType.Str())
	
	// Span 2: Should be processed and renamed
	assert.Equal(t, "POST /test", resultSpans.At(1).Name())
	operationName2, _ := resultSpans.At(1).Attributes().Get("operation.name")
	assert.Equal(t, "POST /test", operationName2.Str())
	operationType2, _ := resultSpans.At(1).Attributes().Get("operation.type")
	assert.Equal(t, "http", operationType2.Str())
	
	// Span 3: Should be processed but operation.type preserved
	assert.Equal(t, "PUT /test", resultSpans.At(2).Name())
	operationName3, _ := resultSpans.At(2).Attributes().Get("operation.name")
	assert.Equal(t, "PUT /test", operationName3.Str())
	operationType3, _ := resultSpans.At(2).Attributes().Get("operation.type")
	assert.Equal(t, "pre-existing-type", operationType3.Str())
}