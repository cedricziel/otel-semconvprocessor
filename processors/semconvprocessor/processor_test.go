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

func TestProcessTraces_AttributeMappings(t *testing.T) {
	tests := []struct {
		name     string
		mappings []AttributeMapping
		input    map[string]string
		expected map[string]string
	}{
		{
			name: "rename attribute",
			mappings: []AttributeMapping{
				{From: "http.method", To: "http.request.method", Action: "rename"},
			},
			input:    map[string]string{"http.method": "GET"},
			expected: map[string]string{"http.request.method": "GET"},
		},
		{
			name: "copy attribute",
			mappings: []AttributeMapping{
				{From: "service.version", To: "service.version.string", Action: "copy"},
			},
			input:    map[string]string{"service.version": "1.0.0"},
			expected: map[string]string{"service.version": "1.0.0", "service.version.string": "1.0.0"},
		},
		{
			name: "move attribute",
			mappings: []AttributeMapping{
				{From: "old.name", To: "new.name", Action: "move"},
			},
			input:    map[string]string{"old.name": "value"},
			expected: map[string]string{"new.name": "value"},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Enabled:  true,
				Mappings: tt.mappings,
			}
			
			telemetryBuilder, _ := metadata.NewTelemetryBuilder(processortest.NewNopSettings(component.MustNewType("semconv")).TelemetrySettings)
			processor := newSemconvProcessor(zap.NewNop(), cfg, telemetryBuilder)
			
			traces := ptrace.NewTraces()
			rs := traces.ResourceSpans().AppendEmpty()
			attrs := rs.Resource().Attributes()
			
			for k, v := range tt.input {
				attrs.PutStr(k, v)
			}
			
			result, err := processor.processTraces(context.Background(), traces)
			require.NoError(t, err)
			
			resultAttrs := result.ResourceSpans().At(0).Resource().Attributes()
			assert.Equal(t, len(tt.expected), resultAttrs.Len())
			
			for k, v := range tt.expected {
				val, exists := resultAttrs.Get(k)
				assert.True(t, exists, "expected attribute %s not found", k)
				assert.Equal(t, v, val.AsString())
			}
		})
	}
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

func TestProcessAttributes_ComplexScenario(t *testing.T) {
	cfg := &Config{
		Enabled: true,
		Mappings: []AttributeMapping{
			{From: "old.attr1", To: "new.attr1", Action: "rename"},
			{From: "old.attr2", To: "new.attr2", Action: "copy"},
			{From: "old.attr3", To: "new.attr3", Action: "move"},
		},
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
	rs.Resource().Attributes().PutStr("old.attr1", "value1")
	rs.Resource().Attributes().PutStr("old.attr2", "value2")
	rs.Resource().Attributes().PutStr("old.attr3", "value3")
	
	ss := rs.ScopeSpans().AppendEmpty()
	span := ss.Spans().AppendEmpty()
	span.SetName("GET /users/12345?include=profile&limit=10")
	
	// Add span attributes
	spanAttrs := span.Attributes()
	spanAttrs.PutStr("http.method", "GET")
	spanAttrs.PutStr("url.path", "/users/12345?include=profile&limit=10")
	spanAttrs.PutStr("old.attr1", "span_value1")
	
	result, err := processor.processTraces(context.Background(), traces)
	require.NoError(t, err)
	
	// Check resource attributes
	resourceAttrs := result.ResourceSpans().At(0).Resource().Attributes()
	_, exists := resourceAttrs.Get("old.attr1")
	assert.False(t, exists, "old.attr1 should be removed")
	
	val, exists := resourceAttrs.Get("new.attr1")
	assert.True(t, exists)
	assert.Equal(t, "value1", val.AsString())
	
	val, exists = resourceAttrs.Get("old.attr2")
	assert.True(t, exists, "old.attr2 should still exist")
	assert.Equal(t, "value2", val.AsString())
	
	val, exists = resourceAttrs.Get("new.attr2")
	assert.True(t, exists)
	assert.Equal(t, "value2", val.AsString())
	
	// Check span
	resultSpan := result.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
	assert.Equal(t, "GET /users/{id}", resultSpan.Name())
	
	// Check span attributes
	resultSpanAttrs := resultSpan.Attributes()
	_, exists = resultSpanAttrs.Get("old.attr1")
	assert.False(t, exists)
	
	val, exists = resultSpanAttrs.Get("new.attr1")
	assert.True(t, exists)
	assert.Equal(t, "span_value1", val.AsString())
}