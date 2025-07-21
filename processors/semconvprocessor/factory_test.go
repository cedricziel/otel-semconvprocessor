// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package semconvprocessor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/processor/processortest"
)

func TestNewFactory(t *testing.T) {
	factory := NewFactory()
	assert.NotNil(t, factory)
	assert.Equal(t, component.MustNewType("semconv"), factory.Type())
}

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NotNil(t, cfg)
	
	defaultCfg, ok := cfg.(*Config)
	require.True(t, ok)
	assert.False(t, defaultCfg.Enabled)
	assert.False(t, defaultCfg.Benchmark)
	assert.Empty(t, defaultCfg.Mappings)
}

func TestCreateTracesProcessor(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	
	tests := []struct {
		name    string
		config  component.Config
		wantErr bool
	}{
		{
			name:    "default config",
			config:  cfg,
			wantErr: false,
		},
		{
			name: "enabled config",
			config: &Config{
				Enabled: true,
			},
			wantErr: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processor, err := createTracesProcessor(
				context.Background(),
				processortest.NewNopSettings(component.MustNewType("semconv")),
				tt.config,
				consumertest.NewNop(),
			)
			
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, processor)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, processor)
			}
		})
	}
}

func TestCreateMetricsProcessor(t *testing.T) {
	cfg := createDefaultConfig()
	
	processor, err := createMetricsProcessor(
		context.Background(),
		processortest.NewNopSettings(component.MustNewType("semconv")),
		cfg,
		consumertest.NewNop(),
	)
	
	assert.NoError(t, err)
	assert.NotNil(t, processor)
}

func TestCreateLogsProcessor(t *testing.T) {
	cfg := createDefaultConfig()
	
	processor, err := createLogsProcessor(
		context.Background(),
		processortest.NewNopSettings(component.MustNewType("semconv")),
		cfg,
		consumertest.NewNop(),
	)
	
	assert.NoError(t, err)
	assert.NotNil(t, processor)
}

func TestFactory_Stability(t *testing.T) {
	// Verify that the stability level is consistent
	assert.Equal(t, component.StabilityLevelAlpha, stability)
}