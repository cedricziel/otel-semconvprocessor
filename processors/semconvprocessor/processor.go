// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package semconvprocessor

import (
	"context"
	"fmt"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
	"go.opentelemetry.io/collector/component"
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
	logger         *zap.Logger
	config         *Config
	telemetry      *metadata.TelemetryBuilder
	compiledRules  []compiledRule
	parser         ottl.Parser[ottlspan.TransformContext]
	spanNameCount  map[string]int64 // For benchmark mode - tracks occurrences
	operationCount map[string]int64 // For benchmark mode - tracks occurrences
}

// compiledRule represents a compiled OTTL rule
type compiledRule struct {
	ID              string
	Priority        int
	SpanKind        []string // Allowed span kinds (empty means all)
	Condition       ottl.Condition[ottlspan.TransformContext]
	OperationName   *ottl.ValueExpression[ottlspan.TransformContext]
	OperationType   *ottl.ValueExpression[ottlspan.TransformContext] // Optional
}

// newSemconvProcessor creates a new semconv processor
func newSemconvProcessor(logger *zap.Logger, config *Config, telemetry *metadata.TelemetryBuilder, set component.TelemetrySettings) (*semconvProcessor, error) {
	sp := &semconvProcessor{
		logger:    logger,
		config:    config,
		telemetry: telemetry,
	}
	
	if config.Benchmark {
		sp.spanNameCount = make(map[string]int64)
		sp.operationCount = make(map[string]int64)
	}
	
	// Initialize OTTL parser if span processing is enabled
	if config.SpanProcessing.Enabled {
		// Create parser with custom functions and telemetry settings
		parser, err := ottlspan.NewParser(
			ottlFunctions[ottlspan.TransformContext](),
			set,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create OTTL parser: %w", err)
		}
		sp.parser = parser
		
		// Compile rules
		if err := sp.compileRules(); err != nil {
			return nil, fmt.Errorf("failed to compile rules: %w", err)
		}
	}
	
	return sp, nil
}

// compileRules compiles OTTL expressions from configuration
func (sp *semconvProcessor) compileRules() error {
	sp.compiledRules = make([]compiledRule, 0, len(sp.config.SpanProcessing.Rules))
	
	for _, rule := range sp.config.SpanProcessing.Rules {
		compiled := compiledRule{
			ID:       rule.ID,
			Priority: rule.Priority,
			SpanKind: rule.SpanKind,
		}
		
		// Compile condition
		condition, err := sp.parser.ParseCondition(rule.Condition)
		if err != nil {
			return fmt.Errorf("failed to parse condition for rule %s: %w", rule.ID, err)
		}
		compiled.Condition = *condition
		
		// Parse operation name as a value expression
		operationName, err := sp.parser.ParseValueExpression(rule.OperationName)
		if err != nil {
			return fmt.Errorf("failed to parse operation_name for rule %s: %w", rule.ID, err)
		}
		compiled.OperationName = operationName
		
		// Parse operation type as a value expression (optional)
		if rule.OperationType != "" {
			operationType, err := sp.parser.ParseValueExpression(rule.OperationType)
			if err != nil {
				return fmt.Errorf("failed to parse operation_type for rule %s: %w", rule.ID, err)
			}
			compiled.OperationType = operationType
		}
		
		sp.compiledRules = append(sp.compiledRules, compiled)
	}
	
	return nil
}

// processTraces processes the incoming traces
func (sp *semconvProcessor) processTraces(ctx context.Context, td ptrace.Traces) (ptrace.Traces, error) {
	if !sp.config.Enabled {
		return td, nil
	}

	start := time.Now()
	spanCount := 0

	// Process traces
	resourceSpans := td.ResourceSpans()
	for i := 0; i < resourceSpans.Len(); i++ {
		rs := resourceSpans.At(i)
		resource := rs.Resource()
		
		scopeSpans := rs.ScopeSpans()
		for j := 0; j < scopeSpans.Len(); j++ {
			ss := scopeSpans.At(j)
			scope := ss.Scope()
			spans := ss.Spans()
			
			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)
				spanCount++
				
				// Process span if rules are enabled
				if sp.config.SpanProcessing.Enabled {
					sp.processSpan(ctx, span, resource, scope)
				}
			}
		}
	}

	// Record metrics
	if spanCount > 0 {
		sp.telemetry.ProcessorSemconvSpansProcessed.Add(ctx, int64(spanCount), 
			metric.WithAttributes(attribute.String("signal_type", "traces")))
	}
	
	// Record benchmark metrics if enabled
	if sp.config.Benchmark {
		sp.recordBenchmarkMetrics(ctx)
	}
	
	duration := float64(time.Since(start).Microseconds()) / 1000.0 // Convert to milliseconds
	sp.telemetry.ProcessorSemconvProcessingDuration.Record(ctx, duration,
		metric.WithAttributes(attribute.String("signal_type", "traces")))

	return td, nil
}

// getSpanKindString converts SpanKind to string for comparison
func getSpanKindString(kind ptrace.SpanKind) string {
	switch kind {
	case ptrace.SpanKindUnspecified:
		return "unspecified"
	case ptrace.SpanKindInternal:
		return "internal"
	case ptrace.SpanKindServer:
		return "server"
	case ptrace.SpanKindClient:
		return "client"
	case ptrace.SpanKindProducer:
		return "producer"
	case ptrace.SpanKindConsumer:
		return "consumer"
	default:
		return "unspecified"
	}
}

// processSpan processes a single span according to configured rules
func (sp *semconvProcessor) processSpan(ctx context.Context, span ptrace.Span, resource pcommon.Resource, scope pcommon.InstrumentationScope) {
	// Track original span name for benchmark mode
	if sp.config.Benchmark {
		if _, exists := sp.spanNameCount[span.Name()]; !exists {
			// First time seeing this span name
			sp.telemetry.ProcessorSemconvUniqueSpanNamesTotal.Add(ctx, 1)
		}
		sp.spanNameCount[span.Name()]++
	}
	
	// Check if operation.name is already set - if so, skip rule evaluation
	if _, exists := span.Attributes().Get(sp.config.SpanProcessing.OperationNameAttribute); exists {
		// Operation name already set, skip processing
		return
	}
	
	// Create OTTL transform context - using dummy values for missing parameters
	dummyScopeSpans := ptrace.NewScopeSpans()
	dummyResourceSpans := ptrace.NewResourceSpans()
	tCtx := ottlspan.NewTransformContext(span, scope, resource, dummyScopeSpans, dummyResourceSpans)
	
	// Evaluate rules in priority order
	for _, rule := range sp.compiledRules {
		// Check span kind restriction if specified
		if len(rule.SpanKind) > 0 {
			spanKindMatches := false
			currentKind := getSpanKindString(span.Kind())
			for _, allowedKind := range rule.SpanKind {
				if allowedKind == currentKind {
					spanKindMatches = true
					break
				}
			}
			if !spanKindMatches {
				continue
			}
		}
		
		// Check condition
		matches, err := rule.Condition.Eval(ctx, tCtx)
		if err != nil {
			sp.logger.Debug("rule condition evaluation error",
				zap.String("rule_id", rule.ID),
				zap.Error(err))
			continue
		}
		
		if !matches {
			continue
		}
		
		// Rule matched - generate operation name
		operationNameVal, err := rule.OperationName.Eval(ctx, tCtx)
		if err != nil {
			sp.logger.Debug("operation name generation error",
				zap.String("rule_id", rule.ID),
				zap.Error(err))
			continue
		}
		
		// Convert to string
		operationName := fmt.Sprintf("%v", operationNameVal)
		
		// Generate operation type if defined
		var operationType string
		if rule.OperationType != nil {
			operationTypeVal, err := rule.OperationType.Eval(ctx, tCtx)
			if err == nil {
				operationType = fmt.Sprintf("%v", operationTypeVal)
			}
		}
		
		// Apply based on mode
		switch sp.config.SpanProcessing.Mode {
		case ModeEnrich:
			// Only add attributes
			span.Attributes().PutStr(sp.config.SpanProcessing.OperationNameAttribute, operationName)
			if operationType != "" {
				// Only set operation.type if not already present
				if _, exists := span.Attributes().Get(sp.config.SpanProcessing.OperationTypeAttribute); !exists {
					span.Attributes().PutStr(sp.config.SpanProcessing.OperationTypeAttribute, operationType)
				}
			}
			
			// Record what would be enforced in enrich mode
			sp.telemetry.ProcessorSemconvSpanNamesEnforced.Add(ctx, 1,
				metric.WithAttributes(
					attribute.String("rule_id", rule.ID),
					attribute.String("operation_type", operationType),
					attribute.String("mode", "enrich"),
				))
			
		case ModeEnforce:
			// Add operation name as attribute
			span.Attributes().PutStr(sp.config.SpanProcessing.OperationNameAttribute, operationName)
			
			// Override span name
			originalName := span.Name()
			if sp.config.SpanProcessing.PreserveOriginalName && originalName != operationName {
				span.Attributes().PutStr(sp.config.SpanProcessing.OriginalNameAttribute, originalName)
			}
			span.SetName(operationName)
			
			// Add operation type as attribute
			if operationType != "" {
				// Only set operation.type if not already present
				if _, exists := span.Attributes().Get(sp.config.SpanProcessing.OperationTypeAttribute); !exists {
					span.Attributes().PutStr(sp.config.SpanProcessing.OperationTypeAttribute, operationType)
				}
			}
			
			// Record actual enforcement
			sp.telemetry.ProcessorSemconvSpanNamesEnforced.Add(ctx, 1,
				metric.WithAttributes(
					attribute.String("rule_id", rule.ID),
					attribute.String("operation_type", operationType),
					attribute.String("mode", "enforce"),
				))
		}
		
		// Track operation name for benchmark mode
		if sp.config.Benchmark {
			if _, exists := sp.operationCount[operationName]; !exists {
				// First time seeing this operation name
				sp.telemetry.ProcessorSemconvUniqueOperationNamesTotal.Add(ctx, 1)
			}
			sp.operationCount[operationName]++
		}
		
		// First match wins - stop processing
		break
	}
}

// processMetrics processes the incoming metrics
func (sp *semconvProcessor) processMetrics(ctx context.Context, md pmetric.Metrics) (pmetric.Metrics, error) {
	if !sp.config.Enabled {
		return md, nil
	}

	start := time.Now()

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

// recordBenchmarkMetrics records cardinality reduction metrics when benchmark mode is enabled
func (sp *semconvProcessor) recordBenchmarkMetrics(ctx context.Context) {
	originalCount := int64(len(sp.spanNameCount))
	reducedCount := int64(len(sp.operationCount))
	
	// Record unique counts (gauges)
	sp.telemetry.ProcessorSemconvOriginalSpanNameCount.Record(ctx, originalCount)
	sp.telemetry.ProcessorSemconvReducedSpanNameCount.Record(ctx, reducedCount)
	
	// Note: Total counts are tracked in processSpan and will be automatically
	// accumulated by the OpenTelemetry metrics SDK as monotonic counters
	
	if originalCount > 0 {
		reduction := float64(originalCount-reducedCount) / float64(originalCount) * 100
		sp.logger.Info("cardinality reduction achieved",
			zap.Int64("original_span_names", originalCount),
			zap.Int64("operation_names", reducedCount),
			zap.Float64("reduction_percentage", reduction))
	}
}