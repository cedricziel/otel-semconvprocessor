// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package semconvprocessor

import (
	"context"

	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

// semconvProcessor is the implementation of the semconv processor
type semconvProcessor struct {
	logger *zap.Logger
	config *Config
}

// newSemconvProcessor creates a new semconv processor
func newSemconvProcessor(logger *zap.Logger, config *Config) *semconvProcessor {
	return &semconvProcessor{
		logger: logger,
		config: config,
	}
}

// processTraces processes the incoming traces
func (sp *semconvProcessor) processTraces(ctx context.Context, td ptrace.Traces) (ptrace.Traces, error) {
	if !sp.config.Enabled {
		return td, nil
	}

	// Process traces here
	// This is where you would implement semantic convention processing for traces
	resourceSpans := td.ResourceSpans()
	for i := 0; i < resourceSpans.Len(); i++ {
		rs := resourceSpans.At(i)
		// Process resource attributes
		sp.processAttributes(rs.Resource().Attributes())
		
		scopeSpans := rs.ScopeSpans()
		for j := 0; j < scopeSpans.Len(); j++ {
			ss := scopeSpans.At(j)
			spans := ss.Spans()
			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)
				// Process span attributes
				sp.processAttributes(span.Attributes())
			}
		}
	}

	return td, nil
}

// processMetrics processes the incoming metrics
func (sp *semconvProcessor) processMetrics(ctx context.Context, md pmetric.Metrics) (pmetric.Metrics, error) {
	if !sp.config.Enabled {
		return md, nil
	}

	// Process metrics here
	// This is where you would implement semantic convention processing for metrics
	resourceMetrics := md.ResourceMetrics()
	for i := 0; i < resourceMetrics.Len(); i++ {
		rm := resourceMetrics.At(i)
		// Process resource attributes
		sp.processAttributes(rm.Resource().Attributes())
	}

	return md, nil
}

// processLogs processes the incoming logs
func (sp *semconvProcessor) processLogs(ctx context.Context, ld plog.Logs) (plog.Logs, error) {
	if !sp.config.Enabled {
		return ld, nil
	}

	// Process logs here
	// This is where you would implement semantic convention processing for logs
	resourceLogs := ld.ResourceLogs()
	for i := 0; i < resourceLogs.Len(); i++ {
		rl := resourceLogs.At(i)
		// Process resource attributes
		sp.processAttributes(rl.Resource().Attributes())
		
		scopeLogs := rl.ScopeLogs()
		for j := 0; j < scopeLogs.Len(); j++ {
			sl := scopeLogs.At(j)
			logs := sl.LogRecords()
			for k := 0; k < logs.Len(); k++ {
				log := logs.At(k)
				// Process log attributes
				sp.processAttributes(log.Attributes())
			}
		}
	}

	return ld, nil
}

// processAttributes applies configured mappings to attributes
func (sp *semconvProcessor) processAttributes(attrs plog.Map) {
	for _, mapping := range sp.config.Mappings {
		switch mapping.Action {
		case "rename":
			if val, exists := attrs.Get(mapping.From); exists {
				attrs.PutStr(mapping.To, val.AsString())
				attrs.Remove(mapping.From)
			}
		case "copy":
			if val, exists := attrs.Get(mapping.From); exists {
				attrs.PutStr(mapping.To, val.AsString())
			}
		case "move":
			if val, exists := attrs.Get(mapping.From); exists {
				attrs.PutStr(mapping.To, val.AsString())
				attrs.Remove(mapping.From)
			}
		default:
			sp.logger.Warn("unknown mapping action", zap.String("action", mapping.Action))
		}
	}
}