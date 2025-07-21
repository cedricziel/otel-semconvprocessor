# OpenTelemetry Collector - Semantic Convention Processor

This repository contains a custom OpenTelemetry Collector processor for handling semantic conventions.

## Structure

- `processors/semconvprocessor/` - The semantic convention processor implementation
- `builder-config.yaml` - Configuration for building the collector with OCB

## Building

1. Install the OpenTelemetry Collector Builder (ocb):
```bash
go install go.opentelemetry.io/collector/cmd/builder@latest
```

2. Build the collector:
```bash
builder --config=builder-config.yaml
```

This will create an `otelcol-semconv` binary with the semconv processor included.

## Configuration

Example collector configuration:

```yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317

processors:
  semconv:
    enabled: true
    mappings:
      - from: "old.attribute.name"
        to: "new.attribute.name"
        action: "rename"

exporters:
  debug:

service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [semconv]
      exporters: [debug]
    metrics:
      receivers: [otlp]
      processors: [semconv]
      exporters: [debug]
    logs:
      receivers: [otlp]
      processors: [semconv]
      exporters: [debug]
```