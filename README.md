# OpenTelemetry Collector - Semantic Convention Processor

A custom OpenTelemetry Collector processor that enforces semantic conventions to reduce cardinality in telemetry data, particularly for span names.

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

The semconv processor enforces semantic conventions and transforms telemetry data to maintain low cardinality. It supports both attribute mappings and span name normalization.

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

### Span Name Rules Configuration

| Field | Type | Description |
|-------|------|-------------|
| `span_name_rules.enabled` | bool | Enables span name enforcement |
| `span_name_rules.http` | object | HTTP-specific span naming rules |
| `span_name_rules.database` | object | Database-specific span naming rules |
| `span_name_rules.messaging` | object | Messaging-specific span naming rules |
| `span_name_rules.custom_rules` | []rule | Custom regex-based transformation rules |

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
    # Attribute mappings for semantic convention migration
    mappings:
      - from: "http.method"
        to: "http.request.method"
        action: "rename"
      - from: "http.status_code"
        to: "http.response.status_code"
        action: "rename"
    
    # Span name enforcement rules to reduce cardinality
    span_name_rules:
      enabled: true
      
      # HTTP span name rules
      http:
        use_url_template: true      # Use url.template or http.route if available
        remove_query_params: true   # Strip query parameters from URLs
        remove_path_params: true    # Replace dynamic path segments with placeholders
      
      # Database span name rules  
      database:
        use_query_summary: true     # Use db.query.summary for span names
        use_operation_name: true    # Use db.operation.name as fallback
      
      # Messaging span name rules
      messaging:
        use_destination_template: true  # Use messaging.destination.template
      
      # Custom transformation rules
      custom_rules:
        - pattern: "^GET /api/users/[0-9]+/profile$"
          replacement: "GET /api/users/{id}/profile"
        - pattern: "^/v[0-9]+/(.*)$"
          replacement: "/v{version}/$1"
          conditions:
            - attribute: "service.name"
              value: "api-gateway"

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

### Cardinality Reduction Examples

The processor transforms high-cardinality span names into low-cardinality equivalents:

| Original Span Name | Transformed Span Name | Rule Applied |
|--------------------|----------------------|---------------|
| `GET /users/12345/profile` | `GET /users/{id}/profile` | HTTP path parameter normalization |
| `GET /search?q=opentelemetry&limit=10` | `GET /search` | Query parameter removal |
| `SELECT * FROM users WHERE id = 12345` | `SELECT users` | Database query summary |
| `publish user.12345.notifications` | `publish user.{id}.notifications` | Custom rule |

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