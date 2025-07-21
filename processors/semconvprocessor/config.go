// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package semconvprocessor

import (
	"errors"
	"fmt"
	"sort"

	"go.opentelemetry.io/collector/component"
)

// Config defines configuration for the semconv processor
type Config struct {
	// Enabled determines if the processor is enabled
	Enabled bool `mapstructure:"enabled"`
	
	// Benchmark enables cardinality metrics tracking
	Benchmark bool `mapstructure:"benchmark"`
	
	// SpanProcessing defines rules for processing span names
	SpanProcessing SpanProcessingConfig `mapstructure:"span_processing"`
}

// SpanProcessingConfig defines configuration for span name processing
type SpanProcessingConfig struct {
	// Enabled determines if span processing is enabled
	Enabled bool `mapstructure:"enabled"`
	
	// Mode: "enrich" (add attributes only) or "enforce" (override span names)
	Mode ProcessingMode `mapstructure:"mode"`
	
	// OperationNameAttribute is the attribute name for generated operation names
	OperationNameAttribute string `mapstructure:"operation_name_attribute"`
	
	// OperationTypeAttribute is the attribute name for operation types
	OperationTypeAttribute string `mapstructure:"operation_type_attribute"`
	
	// PreserveOriginalName determines if original span name should be preserved (enforce mode only)
	PreserveOriginalName bool `mapstructure:"preserve_original_name"`
	
	// OriginalNameAttribute is the attribute name for storing original span name
	OriginalNameAttribute string `mapstructure:"original_name_attribute"`
	
	// Rules defines OTTL rules for span name generation
	Rules []OTTLRule `mapstructure:"rules"`
}

// ProcessingMode defines how span names are processed
type ProcessingMode string

const (
	// ModeEnrich adds operation name as attribute without modifying span name
	ModeEnrich ProcessingMode = "enrich"
	
	// ModeEnforce replaces span name with generated operation name
	ModeEnforce ProcessingMode = "enforce"
)

// OTTLRule defines a single OTTL-based rule for span name generation
type OTTLRule struct {
	// ID is a unique identifier for the rule
	ID string `mapstructure:"id"`
	
	// Priority determines rule evaluation order (lower number = higher priority)
	Priority int `mapstructure:"priority"`
	
	// Condition is an OTTL expression that must evaluate to true for the rule to match
	Condition string `mapstructure:"condition"`
	
	// OperationName is an OTTL expression that generates the operation name
	OperationName string `mapstructure:"operation_name"`
	
	// OperationType is an optional OTTL expression that generates the operation type
	OperationType string `mapstructure:"operation_type"`
}

// Validate checks if the configuration is valid
func (cfg *Config) Validate() error {
	if cfg.SpanProcessing.Enabled {
		if err := cfg.SpanProcessing.Validate(); err != nil {
			return fmt.Errorf("span_processing validation failed: %w", err)
		}
	}
	return nil
}

// Validate checks if the span processing configuration is valid
func (sp *SpanProcessingConfig) Validate() error {
	// Validate mode
	switch sp.Mode {
	case ModeEnrich, ModeEnforce:
		// Valid modes
	case "":
		// Default to enrich if not specified
		sp.Mode = ModeEnrich
	default:
		return fmt.Errorf("invalid mode %q, must be 'enrich' or 'enforce'", sp.Mode)
	}
	
	// Set default attribute names if not specified
	if sp.OperationNameAttribute == "" {
		sp.OperationNameAttribute = "operation.name"
	}
	if sp.OperationTypeAttribute == "" {
		sp.OperationTypeAttribute = "operation.type"
	}
	if sp.OriginalNameAttribute == "" {
		sp.OriginalNameAttribute = "span.name.original"
	}
	
	// Validate rules
	if len(sp.Rules) == 0 {
		return errors.New("at least one rule must be defined")
	}
	
	seenIDs := make(map[string]bool)
	for i, rule := range sp.Rules {
		if rule.ID == "" {
			return fmt.Errorf("rule at index %d has empty ID", i)
		}
		if seenIDs[rule.ID] {
			return fmt.Errorf("duplicate rule ID: %s", rule.ID)
		}
		seenIDs[rule.ID] = true
		
		if rule.Condition == "" {
			return fmt.Errorf("rule %s has empty condition", rule.ID)
		}
		if rule.OperationName == "" {
			return fmt.Errorf("rule %s has empty operation_name", rule.ID)
		}
	}
	
	// Sort rules by priority for consistent evaluation order
	sort.Slice(sp.Rules, func(i, j int) bool {
		return sp.Rules[i].Priority < sp.Rules[j].Priority
	})
	
	return nil
}

var _ component.Config = (*Config)(nil)