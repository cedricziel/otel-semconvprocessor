// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package semconvprocessor

import (
	"go.opentelemetry.io/collector/component"
)

// Config defines configuration for the semconv processor
type Config struct {
	// Enabled determines if the processor is enabled
	Enabled bool `mapstructure:"enabled"`
	
	// Benchmark enables cardinality metrics tracking
	Benchmark bool `mapstructure:"benchmark"`
	
	// Mappings defines attribute mappings for semantic convention processing
	Mappings []AttributeMapping `mapstructure:"mappings"`
	
	// SpanNameRules defines rules for enforcing span name conventions
	SpanNameRules SpanNameRules `mapstructure:"span_name_rules"`
}

// AttributeMapping defines a mapping from one attribute to another
type AttributeMapping struct {
	// From is the source attribute name
	From string `mapstructure:"from"`
	
	// To is the target attribute name
	To string `mapstructure:"to"`
	
	// Action defines what to do with the mapping (e.g., "rename", "copy", "move")
	Action string `mapstructure:"action"`
}

// SpanNameRules defines rules for enforcing low-cardinality span names
type SpanNameRules struct {
	// Enabled determines if span name enforcement is enabled
	Enabled bool `mapstructure:"enabled"`
	
	// HTTPRules defines rules for HTTP span names
	HTTP HTTPSpanNameRules `mapstructure:"http"`
	
	// DatabaseRules defines rules for database span names
	Database DatabaseSpanNameRules `mapstructure:"database"`
	
	// MessagingRules defines rules for messaging span names
	Messaging MessagingSpanNameRules `mapstructure:"messaging"`
	
	// CustomRules defines custom span name transformations
	CustomRules []SpanNameRule `mapstructure:"custom_rules"`
}

// HTTPSpanNameRules defines rules for HTTP span naming
type HTTPSpanNameRules struct {
	// UseURLTemplate uses url.template attribute if available
	UseURLTemplate bool `mapstructure:"use_url_template"`
	
	// RemoveQueryParams removes query parameters from span names
	RemoveQueryParams bool `mapstructure:"remove_query_params"`
	
	// RemovePathParams replaces path parameters with placeholders
	RemovePathParams bool `mapstructure:"remove_path_params"`
}

// DatabaseSpanNameRules defines rules for database span naming
type DatabaseSpanNameRules struct {
	// UseQuerySummary uses db.query.summary if available
	UseQuerySummary bool `mapstructure:"use_query_summary"`
	
	// UseOperationName uses db.operation.name if available
	UseOperationName bool `mapstructure:"use_operation_name"`
}

// MessagingSpanNameRules defines rules for messaging span naming
type MessagingSpanNameRules struct {
	// UseDestinationTemplate uses messaging.destination.template if available
	UseDestinationTemplate bool `mapstructure:"use_destination_template"`
}

// SpanNameRule defines a custom span name transformation rule
type SpanNameRule struct {
	// Pattern is a regex pattern to match span names
	Pattern string `mapstructure:"pattern"`
	
	// Replacement is the replacement pattern (supports regex groups)
	Replacement string `mapstructure:"replacement"`
	
	// Conditions are optional attribute conditions that must be met
	Conditions []Condition `mapstructure:"conditions"`
}

// Condition defines an attribute condition for rule application
type Condition struct {
	// Attribute is the attribute name to check
	Attribute string `mapstructure:"attribute"`
	
	// Value is the expected value (exact match)
	Value string `mapstructure:"value"`
}

// Validate checks if the configuration is valid
func (cfg *Config) Validate() error {
	// Validate custom rules have valid regex patterns
	for _, rule := range cfg.SpanNameRules.CustomRules {
		if rule.Pattern != "" {
			// This will be validated when compiling regex in processor
		}
	}
	return nil
}

var _ component.Config = (*Config)(nil)