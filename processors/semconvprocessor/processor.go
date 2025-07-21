// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package semconvprocessor

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"

	"github.com/cedricziel/semconvprocessor/processors/semconvprocessor/internal/metadata"
)

// semconvProcessor is the implementation of the semconv processor
type semconvProcessor struct {
	logger      *zap.Logger
	config      *Config
	customRules []compiledSpanNameRule
	telemetry   *metadata.TelemetryBuilder
}

// compiledSpanNameRule is a compiled version of SpanNameRule
type compiledSpanNameRule struct {
	pattern     *regexp.Regexp
	replacement string
	conditions  []Condition
}

// newSemconvProcessor creates a new semconv processor
func newSemconvProcessor(logger *zap.Logger, config *Config, telemetry *metadata.TelemetryBuilder) *semconvProcessor {
	sp := &semconvProcessor{
		logger:    logger,
		config:    config,
		telemetry: telemetry,
	}
	
	// Compile custom span name rules
	for _, rule := range config.SpanNameRules.CustomRules {
		if rule.Pattern != "" {
			if re, err := regexp.Compile(rule.Pattern); err == nil {
				sp.customRules = append(sp.customRules, compiledSpanNameRule{
					pattern:     re,
					replacement: rule.Replacement,
					conditions:  rule.Conditions,
				})
			} else {
				logger.Error("failed to compile span name rule pattern",
					zap.String("pattern", rule.Pattern),
					zap.Error(err))
			}
		}
	}
	
	return sp
}

// processTraces processes the incoming traces
func (sp *semconvProcessor) processTraces(ctx context.Context, td ptrace.Traces) (ptrace.Traces, error) {
	if !sp.config.Enabled {
		return td, nil
	}

	start := time.Now()
	spanCount := 0

	// Process traces here
	// This is where you would implement semantic convention processing for traces
	resourceSpans := td.ResourceSpans()
	for i := 0; i < resourceSpans.Len(); i++ {
		rs := resourceSpans.At(i)
		
		scopeSpans := rs.ScopeSpans()
		for j := 0; j < scopeSpans.Len(); j++ {
			ss := scopeSpans.At(j)
			spans := ss.Spans()
			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)
				spanCount++
				// Enforce span name conventions
				if sp.config.SpanNameRules.Enabled {
					sp.enforceSpanName(ctx, span)
				}
			}
		}
	}

	// Record metrics
	if spanCount > 0 {
		sp.telemetry.ProcessorSemconvSpansProcessed.Add(ctx, int64(spanCount), 
			metric.WithAttributes(attribute.String("signal_type", "traces")))
	}
	duration := float64(time.Since(start).Microseconds()) / 1000.0 // Convert to milliseconds
	sp.telemetry.ProcessorSemconvProcessingDuration.Record(ctx, duration,
		metric.WithAttributes(attribute.String("signal_type", "traces")))

	return td, nil
}

// processMetrics processes the incoming metrics
func (sp *semconvProcessor) processMetrics(ctx context.Context, md pmetric.Metrics) (pmetric.Metrics, error) {
	if !sp.config.Enabled {
		return md, nil
	}

	start := time.Now()

	// Process metrics here
	// This is where you would implement semantic convention processing for metrics
	// Process metrics here
	// This is where you would implement semantic convention processing for metrics
	// Currently, this processor focuses on span name enforcement for traces

	duration := float64(time.Since(start).Microseconds()) / 1000.0 // Convert to milliseconds
	sp.telemetry.ProcessorSemconvProcessingDuration.Record(ctx, duration,
		metric.WithAttributes(attribute.String("signal_type", "metrics")))

	return md, nil
}

// processLogs processes the incoming logs
func (sp *semconvProcessor) processLogs(ctx context.Context, ld plog.Logs) (plog.Logs, error) {
	if !sp.config.Enabled {
		return ld, nil
	}

	start := time.Now()

	// Process logs here
	// This is where you would implement semantic convention processing for logs
	resourceLogs := ld.ResourceLogs()
	for i := 0; i < resourceLogs.Len(); i++ {
		rl := resourceLogs.At(i)
		
		scopeLogs := rl.ScopeLogs()
		for j := 0; j < scopeLogs.Len(); j++ {
			sl := scopeLogs.At(j)
			logs := sl.LogRecords()
			for k := 0; k < logs.Len(); k++ {
				// Process log records here
				// This is where you would implement semantic convention processing for logs
			}
		}
	}

	duration := float64(time.Since(start).Microseconds()) / 1000.0 // Convert to milliseconds
	sp.telemetry.ProcessorSemconvProcessingDuration.Record(ctx, duration,
		metric.WithAttributes(attribute.String("signal_type", "logs")))

	return ld, nil
}

// enforceSpanName applies semantic convention rules to span names
func (sp *semconvProcessor) enforceSpanName(ctx context.Context, span ptrace.Span) {
	attrs := span.Attributes()
	originalName := span.Name()
	newName := originalName
	
	// Check span kind and attributes to determine the type
	spanKind := span.Kind()
	
	// HTTP span name enforcement
	if sp.shouldApplyHTTPRules(attrs, spanKind) {
		newName = sp.enforceHTTPSpanName(span, attrs)
		if newName != originalName {
			sp.telemetry.ProcessorSemconvSpanNamesEnforced.Add(ctx, 1,
				metric.WithAttributes(attribute.String("convention_type", "http")))
		}
	}
	
	// Database span name enforcement
	if sp.shouldApplyDatabaseRules(attrs, spanKind) {
		newName = sp.enforceDatabaseSpanName(span, attrs)
		if newName != originalName {
			sp.telemetry.ProcessorSemconvSpanNamesEnforced.Add(ctx, 1,
				metric.WithAttributes(attribute.String("convention_type", "database")))
		}
	}
	
	// Messaging span name enforcement
	if sp.shouldApplyMessagingRules(attrs, spanKind) {
		newName = sp.enforceMessagingSpanName(span, attrs)
		if newName != originalName {
			sp.telemetry.ProcessorSemconvSpanNamesEnforced.Add(ctx, 1,
				metric.WithAttributes(attribute.String("convention_type", "messaging")))
		}
	}
	
	// Apply custom rules
	for _, rule := range sp.customRules {
		if sp.matchesConditions(attrs, rule.conditions) {
			if rule.pattern.MatchString(newName) {
				old := newName
				newName = rule.pattern.ReplaceAllString(newName, rule.replacement)
				if old != newName {
					sp.telemetry.ProcessorSemconvSpanNamesEnforced.Add(ctx, 1,
						metric.WithAttributes(attribute.String("convention_type", "custom")))
				}
			}
		}
	}
	
	// Update span name if changed
	if newName != originalName {
		span.SetName(newName)
		sp.logger.Debug("enforced span name convention",
			zap.String("original", originalName),
			zap.String("new", newName))
	}
}

// shouldApplyHTTPRules checks if HTTP rules should be applied
func (sp *semconvProcessor) shouldApplyHTTPRules(attrs pcommon.Map, kind ptrace.SpanKind) bool {
	if !sp.config.SpanNameRules.HTTP.UseURLTemplate && 
	   !sp.config.SpanNameRules.HTTP.RemoveQueryParams && 
	   !sp.config.SpanNameRules.HTTP.RemovePathParams {
		return false
	}
	
	// Check for HTTP attributes
	if _, ok := attrs.Get("http.method"); ok {
		return true
	}
	if _, ok := attrs.Get("http.request.method"); ok {
		return true
	}
	return false
}

// enforceHTTPSpanName enforces HTTP span naming conventions
func (sp *semconvProcessor) enforceHTTPSpanName(span ptrace.Span, attrs pcommon.Map) string {
	var method, target string
	
	// Get HTTP method
	if val, ok := attrs.Get("http.request.method"); ok {
		method = val.AsString()
	} else if val, ok := attrs.Get("http.method"); ok {
		method = val.AsString()
	}
	
	// Use URL template if available and enabled
	if sp.config.SpanNameRules.HTTP.UseURLTemplate {
		if val, ok := attrs.Get("url.template"); ok {
			target = val.AsString()
		} else if val, ok := attrs.Get("http.route"); ok {
			target = val.AsString()
		}
	}
	
	// If no template, try to extract from URL/path
	if target == "" {
		if val, ok := attrs.Get("url.path"); ok {
			target = val.AsString()
		} else if val, ok := attrs.Get("http.target"); ok {
			target = val.AsString()
		}
		
		// Remove query parameters if configured
		if sp.config.SpanNameRules.HTTP.RemoveQueryParams && target != "" {
			if idx := strings.Index(target, "?"); idx != -1 {
				target = target[:idx]
			}
		}
		
		// Replace path parameters with placeholders if configured
		if sp.config.SpanNameRules.HTTP.RemovePathParams && target != "" {
			target = sp.normalizeHTTPPath(target)
		}
	}
	
	// Format span name according to convention
	if method != "" && target != "" {
		return fmt.Sprintf("%s %s", method, target)
	} else if method != "" {
		return method
	}
	
	return span.Name()
}

// normalizeHTTPPath replaces common path parameters with placeholders
func (sp *semconvProcessor) normalizeHTTPPath(path string) string {
	// Replace UUIDs
	uuidRe := regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)
	path = uuidRe.ReplaceAllString(path, "{id}")
	
	// Replace numeric IDs
	numericRe := regexp.MustCompile(`/\d+(/|$)`)
	path = numericRe.ReplaceAllString(path, "/{id}$1")
	
	return path
}

// shouldApplyDatabaseRules checks if database rules should be applied
func (sp *semconvProcessor) shouldApplyDatabaseRules(attrs pcommon.Map, kind ptrace.SpanKind) bool {
	if !sp.config.SpanNameRules.Database.UseQuerySummary && 
	   !sp.config.SpanNameRules.Database.UseOperationName {
		return false
	}
	
	// Check for database attributes
	if _, ok := attrs.Get("db.system"); ok {
		return true
	}
	return false
}

// enforceDatabaseSpanName enforces database span naming conventions
func (sp *semconvProcessor) enforceDatabaseSpanName(span ptrace.Span, attrs pcommon.Map) string {
	// Use query summary if available and enabled
	if sp.config.SpanNameRules.Database.UseQuerySummary {
		if val, ok := attrs.Get("db.query.summary"); ok {
			return val.AsString()
		}
	}
	
	// Use operation name if available and enabled
	if sp.config.SpanNameRules.Database.UseOperationName {
		if val, ok := attrs.Get("db.operation.name"); ok {
			return val.AsString()
		}
	}
	
	// Fall back to db.name or db.system
	if val, ok := attrs.Get("db.name"); ok {
		return val.AsString()
	}
	if val, ok := attrs.Get("db.system"); ok {
		return val.AsString()
	}
	
	return span.Name()
}

// shouldApplyMessagingRules checks if messaging rules should be applied
func (sp *semconvProcessor) shouldApplyMessagingRules(attrs pcommon.Map, kind ptrace.SpanKind) bool {
	if !sp.config.SpanNameRules.Messaging.UseDestinationTemplate {
		return false
	}
	
	// Check for messaging attributes
	if _, ok := attrs.Get("messaging.system"); ok {
		return true
	}
	return false
}

// enforceMessagingSpanName enforces messaging span naming conventions
func (sp *semconvProcessor) enforceMessagingSpanName(span ptrace.Span, attrs pcommon.Map) string {
	var operation, destination string
	
	// Get operation
	if val, ok := attrs.Get("messaging.operation.type"); ok {
		operation = val.AsString()
	} else if val, ok := attrs.Get("messaging.operation"); ok {
		operation = val.AsString()
	}
	
	// Use destination template if available and enabled
	if sp.config.SpanNameRules.Messaging.UseDestinationTemplate {
		if val, ok := attrs.Get("messaging.destination.template"); ok {
			destination = val.AsString()
		} else if val, ok := attrs.Get("messaging.destination.name"); ok {
			destination = val.AsString()
		}
	}
	
	// Format span name according to convention
	if operation != "" && destination != "" {
		return fmt.Sprintf("%s %s", operation, destination)
	} else if operation != "" {
		return operation
	}
	
	return span.Name()
}

// matchesConditions checks if all conditions are met
func (sp *semconvProcessor) matchesConditions(attrs pcommon.Map, conditions []Condition) bool {
	for _, cond := range conditions {
		if val, ok := attrs.Get(cond.Attribute); ok {
			if val.AsString() != cond.Value {
				return false
			}
		} else {
			return false
		}
	}
	return true
}