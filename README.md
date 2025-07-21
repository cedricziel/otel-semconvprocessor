# OpenTelemetry Collector - Semantic Convention Processor

A custom OpenTelemetry Collector processor that enforces semantic conventions to reduce cardinality in telemetry data using OTTL (OpenTelemetry Transformation Language) for powerful and flexible span name processing.

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

The semconv processor uses OTTL-based rules to reduce cardinality by generating normalized operation names while preserving detailed information in attributes.

### Key Features

- **OTTL-powered rule engine** for maximum flexibility
- **Dual processing modes**: `enrich` (adds attributes) or `enforce` (replaces span names)
- **Rule prioritization** with first-match-wins behavior
- **Custom OTTL functions** for common patterns
- **Cardinality tracking metrics**

### Basic Configuration

```yaml
processors:
  semconv:
    enabled: true
    benchmark: true  # Enable cardinality tracking
    span_processing:
      enabled: true
      mode: "enforce"  # "enrich" or "enforce"
      operation_name_attribute: "operation.name"
      operation_type_attribute: "operation.type" 
      preserve_original_name: true
      original_name_attribute: "span.name.original"
      rules:
        - id: "http_routes"
          priority: 100
          condition: 'attributes["http.method"] != nil and attributes["http.route"] != nil'
          operation_name: 'Concat([attributes["http.method"], attributes["http.route"]], " ")'
          operation_type: '"http"'
```

### Complete Example Configuration

```yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317

processors:
  semconv:
    enabled: true
    benchmark: true
    span_processing:
      enabled: true
      mode: "enforce"
      preserve_original_name: true
      rules:
        # HTTP route normalization (highest priority)
        - id: "http_routes"
          priority: 100
          condition: 'attributes["http.method"] != nil and attributes["http.route"] != nil'
          operation_name: 'Concat([attributes["http.method"], attributes["http.route"]], " ")'
          operation_type: '"http"'
        
        # HTTP path normalization (fallback)
        - id: "http_paths"
          priority: 200
          condition: 'attributes["http.method"] != nil and attributes["url.path"] != nil'
          operation_name: 'Concat([attributes["http.method"], NormalizePath(attributes["url.path"])], " ")'
          operation_type: '"http"'
        
        # Database query processing
        - id: "database_queries"
          priority: 300
          condition: 'attributes["db.statement"] != nil'
          operation_name: 'ParseSQL(attributes["db.statement"])'
          operation_type: 'attributes["db.system"]'
        
        # Messaging operations
        - id: "messaging"
          priority: 400
          condition: 'attributes["messaging.operation"] != nil'
          operation_name: 'Concat([attributes["messaging.operation"], attributes["messaging.destination.name"]], " ")'
          operation_type: '"messaging"'

exporters:
  debug:
    verbosity: detailed

service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [semconv]
      exporters: [debug]
```

### Processing Modes

- **`enrich`**: Adds operation name and type as span attributes, keeps original span names unchanged
- **`enforce`**: Replaces span names with operation names for maximum cardinality reduction

### Custom OTTL Functions

The processor provides specialized OTTL functions for common cardinality reduction patterns:

#### NormalizePath(path)
Normalizes URL paths by replacing identifiers with `{id}` placeholders:
- `/users/12345/profile` → `/users/{id}/profile`
- `/api/orders/550e8400-e29b-41d4-a716-446655440000` → `/api/orders/{id}`

#### ParseSQL(statement)
Extracts operation and primary table from SQL statements:
- `SELECT * FROM users WHERE id = ?` → `SELECT users`
- `INSERT INTO products (name, price) VALUES (?, ?)` → `INSERT products`

#### RemoveQueryParams(url)
Removes query parameters from URLs:
- `/search?q=test&limit=10` → `/search`

### Cardinality Reduction Examples

| Original Span Name | OTTL Rule | Result |
|--------------------|-----------|---------|
| `GET /users/12345/profile` | `NormalizePath(attributes["url.path"])` | `GET /users/{id}/profile` |
| `POST /api/v1/orders/550e8400-e29b-41d4-a716-446655440000` | Custom normalization | `POST /api/v1/orders/{id}` |
| `SELECT * FROM users WHERE id = 12345` | `ParseSQL(attributes["db.statement"])` | `SELECT users` |
| `publish user.created` | Messaging rule | `publish user.created` |

## OTTL Rule Examples

### HTTP Normalizations
```yaml
# Route-based (preferred)
condition: 'attributes["http.method"] != nil and attributes["http.route"] != nil'
operation_name: 'Concat([attributes["http.method"], attributes["http.route"]], " ")'

# Path-based (fallback)  
condition: 'attributes["http.method"] != nil and attributes["url.path"] != nil'
operation_name: 'Concat([attributes["http.method"], NormalizePath(attributes["url.path"])], " ")'

# Query parameter removal
condition: 'attributes["http.target"] != nil'
operation_name: 'Concat([attributes["http.method"], RemoveQueryParams(attributes["http.target"])], " ")'
```

### Database Operations
```yaml
# SQL parsing
condition: 'attributes["db.statement"] != nil'
operation_name: 'ParseSQL(attributes["db.statement"])'

# Simple operation naming
condition: 'attributes["db.operation"] != nil and attributes["db.collection.name"] != nil'
operation_name: 'Concat([attributes["db.operation"], attributes["db.collection.name"]], " ")'
```

### Custom Business Logic
```yaml
# Service-specific rules
condition: 'attributes["service.name"] == "user-service" and span.kind == SPAN_KIND_SERVER'
operation_name: 'Concat([attributes["service.name"], attributes["rpc.method"]], "::")'

# Conditional processing
condition: 'attributes["component"] == "auth" and attributes["operation"] != nil'
operation_name: 'Concat(["auth", attributes["operation"]], ".")'
```

## Telemetry Metrics

Monitor processor effectiveness with built-in metrics:

- `otelcol_processor_semconv_spans_processed` - Total spans processed
- `otelcol_processor_semconv_span_names_enforced` - Span names changed (with `rule_id`)
- `otelcol_processor_semconv_processing_duration` - Processing latency
- `otelcol_processor_semconv_original_span_name_count` - Original cardinality (benchmark mode)
- `otelcol_processor_semconv_reduced_span_name_count` - Reduced cardinality (benchmark mode)

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

### Running Tests with Custom Functions

The processor includes comprehensive tests for all OTTL functionality:
```bash
cd processors/semconvprocessor
go test -v ./... -run TestProcessTraces_CustomFunctions
```

## Migration from Previous Version

**Breaking Change**: The previous attribute mapping functionality has been removed. The new OTTL-based approach provides significantly more flexibility and power. Migrate existing configurations to use OTTL rules instead of the old mapping syntax.

## License

Apache License 2.0