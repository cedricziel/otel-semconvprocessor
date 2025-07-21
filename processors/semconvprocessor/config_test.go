// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package semconvprocessor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
)

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config with OTTL rules",
			config: &Config{
				Enabled: true,
				SpanProcessing: SpanProcessingConfig{
					Enabled: true,
					Mode:    ModeEnrich,
					Rules: []OTTLRule{
						{
							ID:            "test",
							Priority:      100,
							Condition:     `true`,
							OperationName: `"test"`,
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid config enforce mode",
			config: &Config{
				Enabled: true,
				SpanProcessing: SpanProcessingConfig{
					Enabled: true,
					Mode:    ModeEnforce,
					Rules: []OTTLRule{
						{
							ID:            "http",
							Priority:      100,
							Condition:     `attributes["http.method"] != nil`,
							OperationName: `Concat(attributes["http.method"], " ", attributes["http.route"])`,
							OperationType: `"http"`,
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid mode",
			config: &Config{
				Enabled: true,
				SpanProcessing: SpanProcessingConfig{
					Enabled: true,
					Mode:    "invalid",
					Rules: []OTTLRule{
						{
							ID:            "test",
							Priority:      100,
							Condition:     `true`,
							OperationName: `"test"`,
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "invalid mode \"invalid\", must be 'enrich' or 'enforce'",
		},
		{
			name: "no rules",
			config: &Config{
				Enabled: true,
				SpanProcessing: SpanProcessingConfig{
					Enabled: true,
					Mode:    ModeEnrich,
					Rules:   []OTTLRule{},
				},
			},
			wantErr: true,
			errMsg:  "at least one rule must be defined",
		},
		{
			name: "rule with empty ID",
			config: &Config{
				Enabled: true,
				SpanProcessing: SpanProcessingConfig{
					Enabled: true,
					Rules: []OTTLRule{
						{
							ID:            "",
							Priority:      100,
							Condition:     `true`,
							OperationName: `"test"`,
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "rule at index 0 has empty ID",
		},
		{
			name: "duplicate rule IDs",
			config: &Config{
				Enabled: true,
				SpanProcessing: SpanProcessingConfig{
					Enabled: true,
					Rules: []OTTLRule{
						{
							ID:            "test",
							Priority:      100,
							Condition:     `true`,
							OperationName: `"test"`,
						},
						{
							ID:            "test",
							Priority:      200,
							Condition:     `true`,
							OperationName: `"test2"`,
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "duplicate rule ID: test",
		},
		{
			name: "rule with empty condition",
			config: &Config{
				Enabled: true,
				SpanProcessing: SpanProcessingConfig{
					Enabled: true,
					Rules: []OTTLRule{
						{
							ID:            "test",
							Priority:      100,
							Condition:     "",
							OperationName: `"test"`,
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "rule test has empty condition",
		},
		{
			name: "rule with empty operation name",
			config: &Config{
				Enabled: true,
				SpanProcessing: SpanProcessingConfig{
					Enabled: true,
					Rules: []OTTLRule{
						{
							ID:            "test",
							Priority:      100,
							Condition:     `true`,
							OperationName: "",
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "rule test has empty operation_name",
		},
		{
			name: "disabled processor",
			config: &Config{
				Enabled: false,
			},
			wantErr: false,
		},
		{
			name: "empty config defaults to disabled",
			config: &Config{},
			wantErr: false,
		},
		{
			name: "config with benchmark enabled",
			config: &Config{
				Enabled:   true,
				Benchmark: true,
				SpanProcessing: SpanProcessingConfig{
					Enabled: true,
					Rules: []OTTLRule{
						{
							ID:            "test",
							Priority:      100,
							Condition:     `true`,
							OperationName: `"test"`,
						},
					},
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
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
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

func TestSpanProcessingConfig_DefaultValues(t *testing.T) {
	sp := &SpanProcessingConfig{
		Enabled: true,
		Rules: []OTTLRule{
			{
				ID:            "test",
				Priority:      100,
				Condition:     `true`,
				OperationName: `"test"`,
			},
		},
	}
	
	err := sp.Validate()
	assert.NoError(t, err)
	
	// Check defaults were set
	assert.Equal(t, ModeEnrich, sp.Mode)
	assert.Equal(t, "operation.name", sp.OperationNameAttribute)
	assert.Equal(t, "operation.type", sp.OperationTypeAttribute)
	assert.Equal(t, "name.original", sp.OriginalNameAttribute)
}

func TestSpanProcessingConfig_RuleSorting(t *testing.T) {
	sp := &SpanProcessingConfig{
		Enabled: true,
		Mode:    ModeEnrich,
		Rules: []OTTLRule{
			{
				ID:            "low_priority",
				Priority:      1000,
				Condition:     `true`,
				OperationName: `"fallback"`,
			},
			{
				ID:            "high_priority",
				Priority:      100,
				Condition:     `true`,
				OperationName: `"specific"`,
			},
			{
				ID:            "medium_priority",
				Priority:      500,
				Condition:     `true`,
				OperationName: `"medium"`,
			},
		},
	}
	
	err := sp.Validate()
	assert.NoError(t, err)
	
	// Rules should be sorted by priority
	assert.Equal(t, "high_priority", sp.Rules[0].ID)
	assert.Equal(t, "medium_priority", sp.Rules[1].ID)
	assert.Equal(t, "low_priority", sp.Rules[2].ID)
}