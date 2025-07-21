// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package semconvprocessor

import (
	"go.opentelemetry.io/collector/component"
)

// Config defines configuration for the semconv processor
type Config struct {
	// Example configuration fields - customize based on your needs
	
	// Mappings defines attribute mappings for semantic convention processing
	Mappings []AttributeMapping `mapstructure:"mappings"`
	
	// Enabled determines if the processor is enabled
	Enabled bool `mapstructure:"enabled"`
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

// Validate checks if the configuration is valid
func (cfg *Config) Validate() error {
	// Add validation logic here
	// For example, validate that mappings are not empty if enabled
	return nil
}

var _ component.Config = (*Config)(nil)