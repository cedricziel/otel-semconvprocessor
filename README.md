# OpenTelemetry Collector - Semantic Convention Processor

A custom OpenTelemetry Collector processor that transforms semantic conventions in telemetry data.

## Project Structure

- `processors/semconvprocessor/` - Contains the processor implementation
- `builder-config.yaml` - Defines the collector distribution components
- `otelcol-semconv/` - Generated collector distribution (created by OCB)

## Prerequisites

- Go 1.21 or later
- OpenTelemetry Collector Builder (OCB) v0.130.0

## Building

Install the OpenTelemetry Collector Builder:
```bash
go install go.opentelemetry.io/collector/cmd/builder@v0.130.0
```

Build the custom collector:
```bash
ocb --config=builder-config.yaml
```

The build process creates the `otelcol-semconv` binary in the `./otelcol-semconv/` directory.

## Running the Collector

Run the collector with a configuration file:
```bash
./otelcol-semconv/otelcol-semconv --config=config.yaml
```

## Configuration

The semconv processor transforms attributes in traces, metrics, and logs based on configured mappings.

### Processor Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `false` | Enables or disables the processor |
| `mappings` | []mapping | `[]` | List of attribute transformations |

### Mapping Configuration

| Field | Type | Description |
|-------|------|-------------|
| `from` | string | Source attribute name |
| `to` | string | Target attribute name |
| `action` | string | Transformation action: `rename`, `copy`, or `move` |

### Actions

- **rename**: Renames the attribute and removes the original
- **copy**: Copies the attribute value to a new name while preserving the original
- **move**: Alias for rename

### Example Configuration

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
      - from: "http.method"
        to: "http.request.method"
        action: "rename"
      - from: "http.status_code"
        to: "http.response.status_code"
        action: "rename"
      - from: "service.version"
        to: "service.version.string"
        action: "copy"

exporters:
  debug:
    verbosity: detailed

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

## Development

### Testing Changes

Build and test the processor:
```bash
cd processors/semconvprocessor
go test ./...
```

### Updating Dependencies

Update processor dependencies:
```bash
cd processors/semconvprocessor
go get -u ./...
go mod tidy
```

## License

Apache License 2.0