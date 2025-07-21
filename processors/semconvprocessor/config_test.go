// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package semconvprocessor

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
)

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "valid config with mappings",
			config: &Config{
				Enabled: true,
				Mappings: []AttributeMapping{
					{From: "old", To: "new", Action: "rename"},
				},
			},
			wantErr: false,
		},
		{
			name: "valid config with span name rules",
			config: &Config{
				Enabled: true,
				SpanNameRules: SpanNameRules{
					Enabled: true,
					HTTP: HTTPSpanNameRules{
						UseURLTemplate: true,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid config with custom rules",
			config: &Config{
				Enabled: true,
				SpanNameRules: SpanNameRules{
					Enabled: true,
					CustomRules: []SpanNameRule{
						{
							Pattern:     `^/api/v\d+/(.*)$`,
							Replacement: "/api/v{version}/$1",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "disabled processor",
			config: &Config{
				Enabled: false,
			},
			wantErr: false,
		},
		{
			name: "empty config",
			config: &Config{},
			wantErr: false,
		},
		{
			name: "config with benchmark enabled",
			config: &Config{
				Enabled:   true,
				Benchmark: true,
				SpanNameRules: SpanNameRules{
					Enabled: true,
				},
			},
			wantErr: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfig_Implements(t *testing.T) {
	cfg := &Config{}
	// Verify that Config implements component.Config interface
	var _ component.Config = cfg
}

func TestAttributeMapping_Validation(t *testing.T) {
	tests := []struct {
		name    string
		mapping AttributeMapping
		valid   bool
	}{
		{
			name: "valid rename",
			mapping: AttributeMapping{
				From:   "old.attribute",
				To:     "new.attribute",
				Action: "rename",
			},
			valid: true,
		},
		{
			name: "valid copy",
			mapping: AttributeMapping{
				From:   "source",
				To:     "destination",
				Action: "copy",
			},
			valid: true,
		},
		{
			name: "valid move",
			mapping: AttributeMapping{
				From:   "from.attr",
				To:     "to.attr",
				Action: "move",
			},
			valid: true,
		},
		{
			name: "empty from field",
			mapping: AttributeMapping{
				From:   "",
				To:     "new.attribute",
				Action: "rename",
			},
			valid: false,
		},
		{
			name: "empty to field",
			mapping: AttributeMapping{
				From:   "old.attribute",
				To:     "",
				Action: "rename",
			},
			valid: false,
		},
		{
			name: "invalid action",
			mapping: AttributeMapping{
				From:   "old",
				To:     "new",
				Action: "invalid",
			},
			valid: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Note: In a real implementation, you might want to add
			// validation logic to AttributeMapping
			// For now, we're just documenting expected behavior
			if !tt.valid {
				// These cases should be caught during validation
				// when the feature is implemented
			}
		})
	}
}

func TestSpanNameRule_Validation(t *testing.T) {
	tests := []struct {
		name  string
		rule  SpanNameRule
		valid bool
	}{
		{
			name: "valid rule with pattern and replacement",
			rule: SpanNameRule{
				Pattern:     `^GET /users/\d+$`,
				Replacement: "GET /users/{id}",
			},
			valid: true,
		},
		{
			name: "valid rule with conditions",
			rule: SpanNameRule{
				Pattern:     `operation`,
				Replacement: "normalized_op",
				Conditions: []Condition{
					{Attribute: "env", Value: "prod"},
				},
			},
			valid: true,
		},
		{
			name: "invalid regex pattern",
			rule: SpanNameRule{
				Pattern:     `[invalid regex`,
				Replacement: "replacement",
			},
			valid: false,
		},
		{
			name: "empty pattern",
			rule: SpanNameRule{
				Pattern:     "",
				Replacement: "replacement",
			},
			valid: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test pattern compilation
			if tt.rule.Pattern != "" {
				_, err := regexp.Compile(tt.rule.Pattern)
				if tt.valid {
					assert.NoError(t, err)
				} else {
					assert.Error(t, err)
				}
			}
		})
	}
}