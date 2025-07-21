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

func TestProcessTraces_Disabled(t *testing.T) {
	cfg := &Config{
		Enabled: false,
	}
	
	telemetryBuilder, _ := metadata.NewTelemetryBuilder(processortest.NewNopSettings(component.MustNewType("semconv")).TelemetrySettings)
	processor, err := newSemconvProcessor(zap.NewNop(), cfg, telemetryBuilder, processortest.NewNopSettings(component.MustNewType("semconv")).TelemetrySettings)
	require.NoError(t, err)
	
	traces := ptrace.NewTraces()
	result, err := processor.processTraces(context.Background(), traces)
	
	assert.NoError(t, err)
	assert.Equal(t, traces, result)
}

func TestProcessTraces_EnrichMode(t *testing.T) {
	cfg := &Config{
		Enabled: true,
		SpanProcessing: SpanProcessingConfig{
			Enabled: true,
			Mode:    ModeEnrich,
			Rules: []OTTLRule{
				{
					ID:            "http_route",
					Priority:      100,
					Condition:     `attributes["http.method"] != nil and attributes["http.route"] != nil`,
					OperationName: `Concat([attributes["http.method"], attributes["http.route"]], " ")`,
					OperationType: `"http"`,
				},
			},
		},
	}
	// Let validation set defaults
	require.NoError(t, cfg.Validate())
	
	telemetryBuilder, _ := metadata.NewTelemetryBuilder(processortest.NewNopSettings(component.MustNewType("semconv")).TelemetrySettings)
	processor, err := newSemconvProcessor(zap.NewNop(), cfg, telemetryBuilder, processortest.NewNopSettings(component.MustNewType("semconv")).TelemetrySettings)
	require.NoError(t, err)
	
	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()
	span := ss.Spans().AppendEmpty()
	span.SetName("original_name")
	span.Attributes().PutStr("http.method", "GET")
	span.Attributes().PutStr("http.route", "/users/{id}")
	
	result, err := processor.processTraces(context.Background(), traces)
	require.NoError(t, err)
	
	resultSpan := result.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
	
	// In enrich mode, span name should not change
	assert.Equal(t, "original_name", resultSpan.Name())
	
	// Operation name should be added as attribute
	val, exists := resultSpan.Attributes().Get("operation.name")
	assert.True(t, exists)
	assert.Equal(t, "GET /users/{id}", val.AsString())
	
	// Operation type should be added
	val, exists = resultSpan.Attributes().Get("operation.type")
	assert.True(t, exists)
	assert.Equal(t, "http", val.AsString())
}

func TestProcessTraces_EnforceMode(t *testing.T) {
	cfg := &Config{
		Enabled: true,
		SpanProcessing: SpanProcessingConfig{
			Enabled:              true,
			Mode:                 ModeEnforce,
			PreserveOriginalName: true,
			Rules: []OTTLRule{
				{
					ID:            "http_route",
					Priority:      100,
					Condition:     `attributes["http.method"] != nil and attributes["http.route"] != nil`,
					OperationName: `Concat([attributes["http.method"], attributes["http.route"]], " ")`,
					OperationType: `"http"`,
				},
			},
		},
	}
	// Let validation set defaults
	require.NoError(t, cfg.Validate())
	
	telemetryBuilder, _ := metadata.NewTelemetryBuilder(processortest.NewNopSettings(component.MustNewType("semconv")).TelemetrySettings)
	processor, err := newSemconvProcessor(zap.NewNop(), cfg, telemetryBuilder, processortest.NewNopSettings(component.MustNewType("semconv")).TelemetrySettings)
	require.NoError(t, err)
	
	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()
	span := ss.Spans().AppendEmpty()
	span.SetName("original_name")
	span.Attributes().PutStr("http.method", "POST")
	span.Attributes().PutStr("http.route", "/api/users")
	
	result, err := processor.processTraces(context.Background(), traces)
	require.NoError(t, err)
	
	resultSpan := result.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
	
	// In enforce mode, span name should change
	assert.Equal(t, "POST /api/users", resultSpan.Name())
	
	// Original name should be preserved as attribute
	val, exists := resultSpan.Attributes().Get("name.original")
	assert.True(t, exists)
	assert.Equal(t, "original_name", val.AsString())
	
	// Operation type should be added
	val, exists = resultSpan.Attributes().Get("operation.type")
	assert.True(t, exists)
	assert.Equal(t, "http", val.AsString())
}

func TestProcessTraces_SpanKindMatching(t *testing.T) {
	cfg := &Config{
		Enabled: true,
		SpanProcessing: SpanProcessingConfig{
			Enabled: true,
			Mode:    ModeEnforce,
			Rules: []OTTLRule{
				{
					ID:            "http_server",
					Priority:      100,
					SpanKind:      []string{"server"},
					Condition:     `attributes["http.method"] != nil`,
					OperationName: `Concat(["HTTP Server:", attributes["http.method"], attributes["http.route"]], " ")`,
				},
				{
					ID:            "http_client",
					Priority:      200,
					SpanKind:      []string{"client"},
					Condition:     `attributes["http.method"] != nil`,
					OperationName: `Concat(["HTTP Client:", attributes["http.method"], attributes["http.url"]], " ")`,
				},
				{
					ID:            "http_any",
					Priority:      300,
					Condition:     `attributes["http.method"] != nil`,
					OperationName: `"HTTP Generic"`,
				},
			},
		},
	}
	require.NoError(t, cfg.Validate())
	
	telemetryBuilder, _ := metadata.NewTelemetryBuilder(processortest.NewNopSettings(component.MustNewType("semconv")).TelemetrySettings)
	processor, err := newSemconvProcessor(zap.NewNop(), cfg, telemetryBuilder, processortest.NewNopSettings(component.MustNewType("semconv")).TelemetrySettings)
	require.NoError(t, err)
	
	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()
	
	// Server span - should match http_server rule
	serverSpan := ss.Spans().AppendEmpty()
	serverSpan.SetName("original_server")
	serverSpan.SetKind(ptrace.SpanKindServer)
	serverSpan.Attributes().PutStr("http.method", "GET")
	serverSpan.Attributes().PutStr("http.route", "/api/users")
	
	// Client span - should match http_client rule
	clientSpan := ss.Spans().AppendEmpty()
	clientSpan.SetName("original_client")
	clientSpan.SetKind(ptrace.SpanKindClient)
	clientSpan.Attributes().PutStr("http.method", "POST")
	clientSpan.Attributes().PutStr("http.url", "https://api.example.com/data")
	
	// Producer span with HTTP - should match http_any rule (no span_kind restriction)
	producerSpan := ss.Spans().AppendEmpty()
	producerSpan.SetName("original_producer")
	producerSpan.SetKind(ptrace.SpanKindProducer)
	producerSpan.Attributes().PutStr("http.method", "PUT")
	
	result, err := processor.processTraces(context.Background(), traces)
	require.NoError(t, err)
	
	resultSpans := result.ResourceSpans().At(0).ScopeSpans().At(0).Spans()
	
	// Check server span
	assert.Equal(t, "HTTP Server: GET /api/users", resultSpans.At(0).Name())
	
	// Check client span
	assert.Equal(t, "HTTP Client: POST https://api.example.com/data", resultSpans.At(1).Name())
	
	// Check producer span - matched generic rule
	assert.Equal(t, "HTTP Generic", resultSpans.At(2).Name())
}

func TestProcessTraces_RulePriority(t *testing.T) {
	cfg := &Config{
		Enabled: true,
		SpanProcessing: SpanProcessingConfig{
			Enabled: true,
			Mode:    ModeEnforce,
			Rules: []OTTLRule{
				{
					ID:            "fallback",
					Priority:      1000,
					Condition:     `true`,
					OperationName: `"fallback_operation"`,
				},
				{
					ID:            "specific",
					Priority:      100,
					Condition:     `attributes["service.name"] == "test"`,
					OperationName: `"specific_operation"`,
				},
			},
		},
	}
	require.NoError(t, cfg.Validate())
	
	telemetryBuilder, _ := metadata.NewTelemetryBuilder(processortest.NewNopSettings(component.MustNewType("semconv")).TelemetrySettings)
	processor, err := newSemconvProcessor(zap.NewNop(), cfg, telemetryBuilder, processortest.NewNopSettings(component.MustNewType("semconv")).TelemetrySettings)
	require.NoError(t, err)
	
	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()
	span := ss.Spans().AppendEmpty()
	span.SetName("original")
	span.Attributes().PutStr("service.name", "test")
	
	result, err := processor.processTraces(context.Background(), traces)
	require.NoError(t, err)
	
	resultSpan := result.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
	
	// Lower priority number should win
	assert.Equal(t, "specific_operation", resultSpan.Name())
}

func TestProcessTraces_CustomFunctions(t *testing.T) {
	tests := []struct {
		name           string
		rule           OTTLRule
		attributes     map[string]string
		expectedName   string
	}{
		{
			name: "NormalizePath with UUID",
			rule: OTTLRule{
				ID:            "normalize_path",
				Priority:      100,
				Condition:     `attributes["url.path"] != nil`,
				OperationName: `NormalizePath(attributes["url.path"])`,
			},
			attributes: map[string]string{
				"url.path": "/users/550e8400-e29b-41d4-a716-446655440000/profile",
			},
			expectedName: "/users/{id}/profile",
		},
		{
			name: "ParseSQL SELECT",
			rule: OTTLRule{
				ID:            "parse_sql",
				Priority:      100,
				Condition:     `attributes["db.statement"] != nil`,
				OperationName: `ParseSQL(attributes["db.statement"])`,
			},
			attributes: map[string]string{
				"db.statement": "SELECT * FROM users WHERE id = ?",
			},
			expectedName: "SELECT users",
		},
		{
			name: "RemoveQueryParams",
			rule: OTTLRule{
				ID:            "remove_query",
				Priority:      100,
				Condition:     `attributes["http.target"] != nil`,
				OperationName: `RemoveQueryParams(attributes["http.target"])`,
			},
			attributes: map[string]string{
				"http.target": "/search?q=test&limit=10",
			},
			expectedName: "/search",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Enabled: true,
				SpanProcessing: SpanProcessingConfig{
					Enabled: true,
					Mode:    ModeEnforce,
					Rules:   []OTTLRule{tt.rule},
				},
			}
			require.NoError(t, cfg.Validate())
			
			telemetryBuilder, _ := metadata.NewTelemetryBuilder(processortest.NewNopSettings(component.MustNewType("semconv")).TelemetrySettings)
			processor, err := newSemconvProcessor(zap.NewNop(), cfg, telemetryBuilder, processortest.NewNopSettings(component.MustNewType("semconv")).TelemetrySettings)
			require.NoError(t, err)
			
			traces := ptrace.NewTraces()
			rs := traces.ResourceSpans().AppendEmpty()
			ss := rs.ScopeSpans().AppendEmpty()
			span := ss.Spans().AppendEmpty()
			span.SetName("original")
			
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

func TestProcessTraces_Benchmark(t *testing.T) {
	cfg := &Config{
		Enabled:   true,
		Benchmark: true,
		SpanProcessing: SpanProcessingConfig{
			Enabled: true,
			Mode:    ModeEnforce,
			Rules: []OTTLRule{
				{
					ID:            "http",
					Priority:      100,
					Condition:     `attributes["http.method"] != nil`,
					OperationName: `Concat([attributes["http.method"], NormalizePath(attributes["url.path"])], " ")`,
				},
			},
		},
	}
	require.NoError(t, cfg.Validate())
	
	telemetryBuilder, _ := metadata.NewTelemetryBuilder(processortest.NewNopSettings(component.MustNewType("semconv")).TelemetrySettings)
	processor, err := newSemconvProcessor(zap.NewNop(), cfg, telemetryBuilder, processortest.NewNopSettings(component.MustNewType("semconv")).TelemetrySettings)
	require.NoError(t, err)
	
	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()
	
	// Add spans with different user IDs (high cardinality)
	for i := 0; i < 5; i++ {
		span := ss.Spans().AppendEmpty()
		span.SetName("GET /users/12345/profile")
		span.Attributes().PutStr("http.method", "GET")
		span.Attributes().PutStr("url.path", "/users/12345/profile")
	}
	
	// Add more spans with different IDs
	for i := 0; i < 3; i++ {
		span := ss.Spans().AppendEmpty()
		span.SetName("GET /users/67890/profile")
		span.Attributes().PutStr("http.method", "GET")
		span.Attributes().PutStr("url.path", "/users/67890/profile")
	}
	
	_, err = processor.processTraces(context.Background(), traces)
	require.NoError(t, err)
	
	// All spans should be normalized to the same name
	for i := 0; i < ss.Spans().Len(); i++ {
		span := ss.Spans().At(i)
		assert.Equal(t, "GET /users/{id}/profile", span.Name())
	}
	
	// Check benchmark tracking
	assert.Equal(t, 2, len(processor.spanNameCount))  // 2 unique original names
	assert.Equal(t, 1, len(processor.operationCount)) // 1 unique operation name
}

func TestNormalizePath_Patterns(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "/users/550e8400-e29b-41d4-a716-446655440000/profile",
			expected: "/users/{id}/profile",
		},
		{
			input:    "/api/v1/orders/12345/items/67890",
			expected: "/api/v1/orders/{id}/items/{id}",
		},
		{
			input:    "/products/123",
			expected: "/products/{id}",
		},
		{
			input:    "/api/v2/data",
			expected: "/api/v2/data",
		},
		{
			input:    "/users/123/posts/456/comments/789",
			expected: "/users/{id}/posts/{id}/comments/{id}",
		},
		{
			input:    "/objects/507f1f77bcf86cd799439011", // MongoDB ObjectId
			expected: "/objects/{id}",
		},
		{
			input:    "/search?q=test&limit=10",
			expected: "/search",
		},
	}
	
	cfg := &Config{
		Enabled: true,
		SpanProcessing: SpanProcessingConfig{
			Enabled: true,
			Mode:    ModeEnforce,
			Rules: []OTTLRule{
				{
					ID:            "test",
					Priority:      100,
					Condition:     `true`,
					OperationName: `NormalizePath(attributes["path"])`,
				},
			},
		},
	}
	require.NoError(t, cfg.Validate())
	
	telemetryBuilder, _ := metadata.NewTelemetryBuilder(processortest.NewNopSettings(component.MustNewType("semconv")).TelemetrySettings)
	processor, err := newSemconvProcessor(zap.NewNop(), cfg, telemetryBuilder, processortest.NewNopSettings(component.MustNewType("semconv")).TelemetrySettings)
	require.NoError(t, err)
	
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			traces := ptrace.NewTraces()
			rs := traces.ResourceSpans().AppendEmpty()
			ss := rs.ScopeSpans().AppendEmpty()
			span := ss.Spans().AppendEmpty()
			span.SetName("test")
			span.Attributes().PutStr("path", tt.input)
			
			result, err := processor.processTraces(context.Background(), traces)
			require.NoError(t, err)
			
			resultSpan := result.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
			assert.Equal(t, tt.expected, resultSpan.Name())
		})
	}
}

func TestParseSQL_Patterns(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "SELECT * FROM users WHERE id = ?",
			expected: "SELECT users",
		},
		{
			input:    "INSERT INTO products (name, price) VALUES (?, ?)",
			expected: "INSERT products",
		},
		{
			input:    "UPDATE customers SET email = ? WHERE id = ?",
			expected: "UPDATE customers",
		},
		{
			input:    "DELETE FROM orders WHERE created_at < ?",
			expected: "DELETE orders",
		},
		{
			input:    "SELECT u.name FROM `schema`.`users` u JOIN orders o ON u.id = o.user_id",
			expected: "SELECT users",
		},
		{
			input:    "TRUNCATE TABLE sessions",
			expected: "TRUNCATE",
		},
	}
	
	cfg := &Config{
		Enabled: true,
		SpanProcessing: SpanProcessingConfig{
			Enabled: true,
			Mode:    ModeEnforce,
			Rules: []OTTLRule{
				{
					ID:            "test",
					Priority:      100,
					Condition:     `true`,
					OperationName: `ParseSQL(attributes["sql"])`,
				},
			},
		},
	}
	require.NoError(t, cfg.Validate())
	
	telemetryBuilder, _ := metadata.NewTelemetryBuilder(processortest.NewNopSettings(component.MustNewType("semconv")).TelemetrySettings)
	processor, err := newSemconvProcessor(zap.NewNop(), cfg, telemetryBuilder, processortest.NewNopSettings(component.MustNewType("semconv")).TelemetrySettings)
	require.NoError(t, err)
	
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			traces := ptrace.NewTraces()
			rs := traces.ResourceSpans().AppendEmpty()
			ss := rs.ScopeSpans().AppendEmpty()
			span := ss.Spans().AppendEmpty()
			span.SetName("test")
			span.Attributes().PutStr("sql", tt.input)
			
			result, err := processor.processTraces(context.Background(), traces)
			require.NoError(t, err)
			
			resultSpan := result.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
			assert.Equal(t, tt.expected, resultSpan.Name())
		})
	}
}