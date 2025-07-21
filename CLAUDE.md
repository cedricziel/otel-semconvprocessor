# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is an OpenTelemetry Collector project that implements a custom processor called `semconvprocessor` for transforming semantic conventions in telemetry data. The project uses the OpenTelemetry Collector Builder (OCB) to create a custom collector distribution.

## Build Commands

```bash
# Build the custom collector (from project root)
ocb --config=builder-config.yaml

# The built binary will be at: ./otelcol-semconv/otelcol-semconv
```

## Development Commands

```bash
# Run go mod tidy in the processor directory
cd processors/semconvprocessor && go mod tidy

# Test the processor build
cd processors/semconvprocessor && go build .

# Run the built collector with a config file
./otelcol-semconv/otelcol-semconv --config=config.yaml
```

## Architecture

### Project Structure
- **processors/semconvprocessor/**: Contains the processor implementation
  - `factory.go`: Factory pattern implementation for creating processor instances
  - `config.go`: Configuration structures with mapstructure tags
  - `processor.go`: Core processor logic that handles traces, metrics, and logs
  
- **builder-config.yaml**: OCB manifest that defines which components to include in the custom collector

### Key Design Patterns

1. **Processor Implementation**: The processor implements separate processing functions for traces, metrics, and logs, all sharing a common `processAttributes` method that applies the configured mappings.

2. **Attribute Mappings**: Supports three actions:
   - `rename`: Renames an attribute and removes the old one
   - `copy`: Copies an attribute value to a new name, keeping the original
   - `move`: Alias for rename

3. **OpenTelemetry API Usage**:
   - Uses `pcommon.Map` for attribute maps (not `plog.Map`)
   - Uses `processorhelper.NewTraces/NewMetrics/NewLogs` (not the older `NewTracesProcessor` variants)
   - Compatible with collector v0.130.0 and pdata v1.36.0

## Important Notes

- When updating dependencies, ensure compatibility between collector components and pdata versions
- The processor is disabled by default (`enabled: false`) and must be explicitly enabled in configuration
- The OCB version should match the `otelcol_version` specified in builder-config.yaml