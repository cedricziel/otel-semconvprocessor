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
	processor := newSemconvProcessor(zap.NewNop(), cfg, telemetryBuilder)
	
	traces := ptrace.NewTraces()
	result, err := processor.processTraces(context.Background(), traces)
	
	assert.NoError(t, err)
	assert.Equal(t, traces, result)
}

func TestEnforceSpanName_HTTP(t *testing.T) {
	tests := []struct {
		name         string
		config       HTTPSpanNameRules
		spanName     string
		attributes   map[string]string
		expectedName string
	}{
		{
			name: "use url template",
			config: HTTPSpanNameRules{
				UseURLTemplate: true,
			},
			spanName: "GET /users/12345",
			attributes: map[string]string{
				"http.method":  "GET",
				"url.template": "/users/{id}",
			},
			expectedName: "GET /users/{id}",
		},
		{
			name: "use http route",
			config: HTTPSpanNameRules{
				UseURLTemplate: true,
			},
			spanName: "GET /api/v1/users/12345",
			attributes: map[string]string{
				"http.request.method": "GET",
				"http.route":          "/api/v1/users/:id",
			},
			expectedName: "GET /api/v1/users/:id",
		},
		{
			name: "remove query params",
			config: HTTPSpanNameRules{
				RemoveQueryParams: true,
			},
			spanName: "GET /search?q=test&limit=10",
			attributes: map[string]string{
				"http.method": "GET",
				"url.path":    "/search?q=test&limit=10",
			},
			expectedName: "GET /search",
		},
		{
			name: "normalize path params - UUID",
			config: HTTPSpanNameRules{
				RemovePathParams: true,
			},
			spanName: "GET /users/550e8400-e29b-41d4-a716-446655440000/profile",
			attributes: map[string]string{
				"http.method": "GET",
				"url.path":    "/users/550e8400-e29b-41d4-a716-446655440000/profile",
			},
			expectedName: "GET /users/{id}/profile",
		},
		{
			name: "normalize path params - numeric",
			config: HTTPSpanNameRules{
				RemovePathParams: true,
			},
			spanName: "GET /api/v2/items/12345/details",
			attributes: map[string]string{
				"http.method": "GET",
				"url.path":    "/api/v2/items/12345/details",
			},
			expectedName: "GET /api/v2/items/{id}/details",
		},
		{
			name: "method only when no target",
			config: HTTPSpanNameRules{
				UseURLTemplate: true,
			},
			spanName: "some_operation",
			attributes: map[string]string{
				"http.method": "POST",
			},
			expectedName: "POST",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Enabled: true,
				SpanNameRules: SpanNameRules{
					Enabled: true,
					HTTP:    tt.config,
				},
			}
			
			telemetryBuilder, _ := metadata.NewTelemetryBuilder(processortest.NewNopSettings(component.MustNewType("semconv")).TelemetrySettings)
			processor := newSemconvProcessor(zap.NewNop(), cfg, telemetryBuilder)
			
			span := ptrace.NewSpan()
			span.SetName(tt.spanName)
			attrs := span.Attributes()
			
			for k, v := range tt.attributes {
				attrs.PutStr(k, v)
			}
			
			processor.enforceSpanName(context.Background(), span)
			assert.Equal(t, tt.expectedName, span.Name())
		})
	}
}

func TestEnforceSpanName_Database(t *testing.T) {
	tests := []struct {
		name         string
		config       DatabaseSpanNameRules
		spanName     string
		attributes   map[string]string
		expectedName string
	}{
		{
			name: "use query summary",
			config: DatabaseSpanNameRules{
				UseQuerySummary: true,
			},
			spanName: "SELECT * FROM users WHERE id = ?",
			attributes: map[string]string{
				"db.system":        "postgresql",
				"db.query.summary": "SELECT users",
			},
			expectedName: "SELECT users",
		},
		{
			name: "use operation name",
			config: DatabaseSpanNameRules{
				UseOperationName: true,
			},
			spanName: "complex query",
			attributes: map[string]string{
				"db.system":         "mysql",
				"db.operation.name": "FindUserById",
			},
			expectedName: "FindUserById",
		},
		{
			name: "fallback to db.name",
			config: DatabaseSpanNameRules{
				UseQuerySummary:  true,
				UseOperationName: true,
			},
			spanName: "some query",
			attributes: map[string]string{
				"db.system": "redis",
				"db.name":   "cache",
			},
			expectedName: "cache",
		},
		{
			name: "fallback to db.system",
			config: DatabaseSpanNameRules{
				UseQuerySummary: true,
			},
			spanName: "query",
			attributes: map[string]string{
				"db.system": "mongodb",
			},
			expectedName: "mongodb",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Enabled: true,
				SpanNameRules: SpanNameRules{
					Enabled:  true,
					Database: tt.config,
				},
			}
			
			telemetryBuilder, _ := metadata.NewTelemetryBuilder(processortest.NewNopSettings(component.MustNewType("semconv")).TelemetrySettings)
			processor := newSemconvProcessor(zap.NewNop(), cfg, telemetryBuilder)
			
			span := ptrace.NewSpan()
			span.SetName(tt.spanName)
			attrs := span.Attributes()
			
			for k, v := range tt.attributes {
				attrs.PutStr(k, v)
			}
			
			processor.enforceSpanName(context.Background(), span)
			assert.Equal(t, tt.expectedName, span.Name())
		})
	}
}

func TestEnforceSpanName_Messaging(t *testing.T) {
	tests := []struct {
		name         string
		config       MessagingSpanNameRules
		spanName     string
		attributes   map[string]string
		expectedName string
	}{
		{
			name: "use destination template",
			config: MessagingSpanNameRules{
				UseDestinationTemplate: true,
			},
			spanName: "publish to dynamic topic",
			attributes: map[string]string{
				"messaging.system":                "kafka",
				"messaging.operation.type":        "publish",
				"messaging.destination.template":  "orders.{region}.events",
			},
			expectedName: "publish orders.{region}.events",
		},
		{
			name: "use destination name as fallback",
			config: MessagingSpanNameRules{
				UseDestinationTemplate: true,
			},
			spanName: "consume",
			attributes: map[string]string{
				"messaging.system":           "rabbitmq",
				"messaging.operation":        "consume",
				"messaging.destination.name": "user.notifications",
			},
			expectedName: "consume user.notifications",
		},
		{
			name: "operation only when no destination",
			config: MessagingSpanNameRules{
				UseDestinationTemplate: true,
			},
			spanName: "messaging operation",
			attributes: map[string]string{
				"messaging.system":    "sqs",
				"messaging.operation": "send",
			},
			expectedName: "send",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Enabled: true,
				SpanNameRules: SpanNameRules{
					Enabled:   true,
					Messaging: tt.config,
				},
			}
			
			telemetryBuilder, _ := metadata.NewTelemetryBuilder(processortest.NewNopSettings(component.MustNewType("semconv")).TelemetrySettings)
			processor := newSemconvProcessor(zap.NewNop(), cfg, telemetryBuilder)
			
			span := ptrace.NewSpan()
			span.SetName(tt.spanName)
			attrs := span.Attributes()
			
			for k, v := range tt.attributes {
				attrs.PutStr(k, v)
			}
			
			processor.enforceSpanName(context.Background(), span)
			assert.Equal(t, tt.expectedName, span.Name())
		})
	}
}

func TestEnforceSpanName_CustomRules(t *testing.T) {
	tests := []struct {
		name         string
		rules        []SpanNameRule
		spanName     string
		attributes   map[string]string
		expectedName string
	}{
		{
			name: "simple pattern replacement",
			rules: []SpanNameRule{
				{
					Pattern:     `^GET /api/users/\d+/profile$`,
					Replacement: "GET /api/users/{id}/profile",
				},
			},
			spanName:     "GET /api/users/12345/profile",
			attributes:   map[string]string{},
			expectedName: "GET /api/users/{id}/profile",
		},
		{
			name: "pattern with capture groups",
			rules: []SpanNameRule{
				{
					Pattern:     `^/v(\d+)/(.*)$`,
					Replacement: "/v{version}/$2",
				},
			},
			spanName:     "/v2/users/list",
			attributes:   map[string]string{},
			expectedName: "/v{version}/users/list",
		},
		{
			name: "rule with conditions met",
			rules: []SpanNameRule{
				{
					Pattern:     `user\.(\d+)\.notifications`,
					Replacement: "user.{id}.notifications",
					Conditions: []Condition{
						{Attribute: "service.name", Value: "notification-service"},
					},
				},
			},
			spanName: "user.12345.notifications",
			attributes: map[string]string{
				"service.name": "notification-service",
			},
			expectedName: "user.{id}.notifications",
		},
		{
			name: "rule with conditions not met",
			rules: []SpanNameRule{
				{
					Pattern:     `user\.(\d+)\.notifications`,
					Replacement: "user.{id}.notifications",
					Conditions: []Condition{
						{Attribute: "service.name", Value: "notification-service"},
					},
				},
			},
			spanName: "user.12345.notifications",
			attributes: map[string]string{
				"service.name": "other-service",
			},
			expectedName: "user.12345.notifications", // unchanged
		},
		{
			name: "multiple conditions all must match",
			rules: []SpanNameRule{
				{
					Pattern:     `operation`,
					Replacement: "normalized_operation",
					Conditions: []Condition{
						{Attribute: "env", Value: "prod"},
						{Attribute: "region", Value: "us-east-1"},
					},
				},
			},
			spanName: "operation",
			attributes: map[string]string{
				"env":    "prod",
				"region": "us-east-1",
			},
			expectedName: "normalized_operation",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Enabled: true,
				SpanNameRules: SpanNameRules{
					Enabled:     true,
					CustomRules: tt.rules,
				},
			}
			
			telemetryBuilder, _ := metadata.NewTelemetryBuilder(processortest.NewNopSettings(component.MustNewType("semconv")).TelemetrySettings)
			processor := newSemconvProcessor(zap.NewNop(), cfg, telemetryBuilder)
			
			span := ptrace.NewSpan()
			span.SetName(tt.spanName)
			attrs := span.Attributes()
			
			for k, v := range tt.attributes {
				attrs.PutStr(k, v)
			}
			
			processor.enforceSpanName(context.Background(), span)
			assert.Equal(t, tt.expectedName, span.Name())
		})
	}
}

func TestProcessTraces_BenchmarkMetrics(t *testing.T) {
	cfg := &Config{
		Enabled:   true,
		Benchmark: true,
		SpanNameRules: SpanNameRules{
			Enabled: true,
			HTTP: HTTPSpanNameRules{
				RemovePathParams: true,
			},
		},
	}
	
	telemetryBuilder, _ := metadata.NewTelemetryBuilder(processortest.NewNopSettings(component.MustNewType("semconv")).TelemetrySettings)
	processor := newSemconvProcessor(zap.NewNop(), cfg, telemetryBuilder)
	
	// Create traces with multiple spans having high-cardinality names
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
	
	_, err := processor.processTraces(context.Background(), traces)
	require.NoError(t, err)
	
	// Verify all spans have been normalized to the same name
	for i := 0; i < ss.Spans().Len(); i++ {
		span := ss.Spans().At(i)
		assert.Equal(t, "GET /users/{id}/profile", span.Name())
	}
	
	// Note: In a real test, we would verify the metrics were recorded
	// but that requires more complex setup with a metrics exporter
}

func TestNormalizeHTTPPath(t *testing.T) {
	processor := &semconvProcessor{}
	
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
	}
	
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := processor.normalizeHTTPPath(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestProcessTraces_ComplexScenario(t *testing.T) {
	cfg := &Config{
		Enabled: true,
		SpanNameRules: SpanNameRules{
			Enabled: true,
			HTTP: HTTPSpanNameRules{
				UseURLTemplate:    true,
				RemoveQueryParams: true,
				RemovePathParams:  true,
			},
		},
	}
	
	telemetryBuilder, _ := metadata.NewTelemetryBuilder(processortest.NewNopSettings(component.MustNewType("semconv")).TelemetrySettings)
	processor := newSemconvProcessor(zap.NewNop(), cfg, telemetryBuilder)
	
	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	
	// Add resource attributes
	rs.Resource().Attributes().PutStr("service.name", "test-service")
	rs.Resource().Attributes().PutStr("service.version", "1.0.0")
	
	ss := rs.ScopeSpans().AppendEmpty()
	span := ss.Spans().AppendEmpty()
	span.SetName("GET /users/12345?include=profile&limit=10")
	
	// Add span attributes
	spanAttrs := span.Attributes()
	spanAttrs.PutStr("http.method", "GET")
	spanAttrs.PutStr("url.path", "/users/12345?include=profile&limit=10")
	spanAttrs.PutStr("user.id", "12345")
	
	result, err := processor.processTraces(context.Background(), traces)
	require.NoError(t, err)
	
	// Check resource attributes remain unchanged
	resourceAttrs := result.ResourceSpans().At(0).Resource().Attributes()
	val, exists := resourceAttrs.Get("service.name")
	assert.True(t, exists)
	assert.Equal(t, "test-service", val.AsString())
	
	val, exists = resourceAttrs.Get("service.version")
	assert.True(t, exists)
	assert.Equal(t, "1.0.0", val.AsString())
	
	// Check span name is normalized
	resultSpan := result.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
	assert.Equal(t, "GET /users/{id}", resultSpan.Name())
	
	// Check span attributes remain unchanged
	resultSpanAttrs := resultSpan.Attributes()
	val, exists = resultSpanAttrs.Get("http.method")
	assert.True(t, exists)
	assert.Equal(t, "GET", val.AsString())
	
	val, exists = resultSpanAttrs.Get("user.id")
	assert.True(t, exists)
	assert.Equal(t, "12345", val.AsString())
}