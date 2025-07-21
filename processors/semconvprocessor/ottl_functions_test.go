// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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

func TestProcessTraces_FirstNonNil(t *testing.T) {
	cfg := &Config{
		Enabled: true,
		SpanProcessing: SpanProcessingConfig{
			Enabled:                true,
			Mode:                   ModeEnforce,
			OperationNameAttribute: "operation.name",
			OperationTypeAttribute: "operation.type",
			Rules: []OTTLRule{
				{
					ID:            "http_first_non_nil",
					Priority:      100,
					Condition:     `FirstNonNil([attributes["http.request.method"], attributes["http.method"]]) != nil`,
					OperationName: `Concat([FirstNonNil([attributes["http.request.method"], attributes["http.method"]]), " /api"], "")`,
					OperationType: `"http"`,
				},
			},
		},
	}
	
	telemetryBuilder, _ := metadata.NewTelemetryBuilder(processortest.NewNopSettings(component.MustNewType("semconv")).TelemetrySettings)
	processor, err := newSemconvProcessor(zap.NewNop(), cfg, telemetryBuilder, processortest.NewNopSettings(component.MustNewType("semconv")).TelemetrySettings)
	require.NoError(t, err)
	
	tests := []struct {
		name         string
		attributes   map[string]string
		expectedName string
	}{
		{
			name: "new attribute exists",
			attributes: map[string]string{
				"http.request.method": "GET",
				"http.method":         "POST", // Should be ignored
			},
			expectedName: "GET /api",
		},
		{
			name: "only old attribute exists",
			attributes: map[string]string{
				"http.method": "POST",
			},
			expectedName: "POST /api",
		},
		{
			name: "neither attribute exists",
			attributes: map[string]string{
				"some.other": "value",
			},
			expectedName: "test", // Original name unchanged
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			traces := ptrace.NewTraces()
			rs := traces.ResourceSpans().AppendEmpty()
			ss := rs.ScopeSpans().AppendEmpty()
			span := ss.Spans().AppendEmpty()
			span.SetName("test")
			
			// Add attributes
			for k, v := range tt.attributes {
				span.Attributes().PutStr(k, v)
			}
			
			result, err := processor.processTraces(context.Background(), traces)
			require.NoError(t, err)
			
			resultSpan := result.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
			assert.Equal(t, tt.expectedName, resultSpan.Name())
		})
	}
}

func TestFirstNonNil_MultipleAttributes(t *testing.T) {
	cfg := &Config{
		Enabled: true,
		SpanProcessing: SpanProcessingConfig{
			Enabled:                true,
			Mode:                   ModeEnforce,
			OperationNameAttribute: "operation.name",
			OperationTypeAttribute: "operation.type",
			Rules: []OTTLRule{
				{
					ID:            "multiple_fallback",
					Priority:      100,
					Condition:     `FirstNonNil([attributes["preferred"], attributes["secondary"], attributes["fallback"]]) != nil`,
					OperationName: `FirstNonNil([attributes["preferred"], attributes["secondary"], attributes["fallback"]])`,
					OperationType: `"test"`,
				},
			},
		},
	}
	
	telemetryBuilder, _ := metadata.NewTelemetryBuilder(processortest.NewNopSettings(component.MustNewType("semconv")).TelemetrySettings)
	processor, err := newSemconvProcessor(zap.NewNop(), cfg, telemetryBuilder, processortest.NewNopSettings(component.MustNewType("semconv")).TelemetrySettings)
	require.NoError(t, err)
	
	tests := []struct {
		name         string
		attributes   map[string]string
		expectedName string
	}{
		{
			name: "preferred exists",
			attributes: map[string]string{
				"preferred": "first-choice",
				"secondary": "second-choice",
				"fallback":  "last-choice",
			},
			expectedName: "first-choice",
		},
		{
			name: "only secondary exists",
			attributes: map[string]string{
				"secondary": "second-choice",
				"fallback":  "last-choice",
			},
			expectedName: "second-choice",
		},
		{
			name: "only fallback exists",
			attributes: map[string]string{
				"fallback": "last-choice",
			},
			expectedName: "last-choice",
		},
		{
			name:         "none exist",
			attributes:   map[string]string{},
			expectedName: "test", // Original name unchanged
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			traces := ptrace.NewTraces()
			rs := traces.ResourceSpans().AppendEmpty()
			ss := rs.ScopeSpans().AppendEmpty()
			span := ss.Spans().AppendEmpty()
			span.SetName("test")
			
			// Add attributes
			for k, v := range tt.attributes {
				span.Attributes().PutStr(k, v)
			}
			
			result, err := processor.processTraces(context.Background(), traces)
			require.NoError(t, err)
			
			resultSpan := result.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
			assert.Equal(t, tt.expectedName, resultSpan.Name())
		})
	}
}