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

### Local Binary

Run the collector with a configuration file:
```bash
./otelcol-semconv/otelcol-semconv --config=config.yaml
```

### Docker

Build and run the collector using Docker:

```bash
# Build the Docker image
docker build -t otelcol-semconv:latest .

# Run the container
docker run -d \
  --name otelcol-semconv \
  -p 4317:4317 \
  -p 4318:4318 \
  -p 8888:8888 \
  -v $(pwd)/config.yaml:/etc/otelcol/config.yaml:ro \
  otelcol-semconv:latest

# Or use docker-compose for a complete setup with Jaeger
docker-compose up -d
```

The Docker image includes:
- Multi-stage build for minimal image size
- Non-root user for security
- Health check endpoint
- All standard OTLP ports exposed
- Volume mount for configuration

## Configuration

The semconv processor uses OTTL-based rules to reduce cardinality by generating normalized operation names while preserving detailed information in attributes.

### Key Features

- **OTTL-powered rule engine** for maximum flexibility
- **Dual processing modes**: `enrich` (adds attributes) or `enforce` (replaces span names)
- **Rule prioritization** with first-match-wins behavior
- **Span kind filtering** to create targeted rules for specific span types
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
      original_name_attribute: "name.original"
      rules:
        - id: "http_server_routes"
          priority: 100
          span_kind: ["server"]  # Only matches server spans
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
        # HTTP server spans - route normalization (highest priority)
        - id: "http_server_routes"
          priority: 100
          span_kind: ["server"]  # Only server spans
          condition: 'attributes["http.method"] != nil and attributes["http.route"] != nil'
          operation_name: 'Concat([attributes["http.method"], attributes["http.route"]], " ")'
          operation_type: '"http"'
        
        # HTTP client spans - URL normalization
        - id: "http_client_requests"
          priority: 150
          span_kind: ["client"]  # Only client spans
          condition: 'attributes["http.method"] != nil and attributes["http.url"] != nil'
          operation_name: 'Concat([attributes["http.method"], RemoveQueryParams(attributes["http.url"])], " ")'
          operation_type: '"http_client"'
        
        # HTTP path normalization (fallback for any span kind)
        - id: "http_paths"
          priority: 200
          condition: 'attributes["http.method"] != nil and attributes["url.path"] != nil'
          operation_name: 'Concat([attributes["http.method"], NormalizePath(attributes["url.path"])], " ")'
          operation_type: '"http"'
        
        # Database client operations
        - id: "database_queries"
          priority: 300
          span_kind: ["client"]  # Database calls are typically client spans
          condition: 'attributes["db.statement"] != nil'
          operation_name: 'ParseSQL(attributes["db.statement"])'
          operation_type: 'attributes["db.system"]'
        
        # Messaging producer operations
        - id: "messaging_producer"
          priority: 400
          span_kind: ["producer"]
          condition: 'attributes["messaging.operation"] != nil and attributes["messaging.destination.name"] != nil'
          operation_name: 'Concat([attributes["messaging.operation"], attributes["messaging.destination.name"]], " ")'
          operation_type: '"messaging"'
        
        # Messaging consumer operations
        - id: "messaging_consumer"
          priority: 450
          span_kind: ["consumer"]
          condition: 'attributes["messaging.operation"] != nil and attributes["messaging.destination.name"] != nil'
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

### Span Kind Filtering

Rules can be restricted to specific span kinds to create more targeted transformations:

```yaml
# Server-only rule
- id: "api_endpoints"
  priority: 100
  span_kind: ["server"]  # Only matches server spans
  condition: 'attributes["http.route"] != nil'
  operation_name: 'attributes["http.route"]'

# Client-only rule  
- id: "http_clients"
  priority: 150
  span_kind: ["client"]  # Only matches client spans
  condition: 'attributes["http.url"] != nil'
  operation_name: 'RemoveQueryParams(attributes["http.url"])'

# Multiple span kinds
- id: "async_operations"
  priority: 200
  span_kind: ["producer", "consumer"]  # Matches either producer OR consumer spans
  condition: 'attributes["messaging.system"] != nil'
  operation_name: 'Concat([attributes["messaging.system"], attributes["messaging.operation"]], ".")'

# No span_kind - matches all spans
- id: "catch_all"
  priority: 1000
  condition: 'true'
  operation_name: '"unknown"'
```

### HTTP Normalizations
```yaml
# Server spans - route-based (preferred)
span_kind: ["server"]
condition: 'attributes["http.method"] != nil and attributes["http.route"] != nil'
operation_name: 'Concat([attributes["http.method"], attributes["http.route"]], " ")'

# Client spans - URL-based
span_kind: ["client"]
condition: 'attributes["http.method"] != nil and attributes["http.url"] != nil'
operation_name: 'Concat([attributes["http.method"], RemoveQueryParams(attributes["http.url"])], " ")'

# Any span kind - path-based (fallback)  
condition: 'attributes["http.method"] != nil and attributes["url.path"] != nil'
operation_name: 'Concat([attributes["http.method"], NormalizePath(attributes["url.path"])], " ")'
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

### Self-Monitoring

The collector configuration includes self-monitoring capabilities:

1. **Prometheus Receiver**: Scrapes the collector's own metrics endpoint
2. **Resource Processor**: Enriches internal metrics with collector metadata
3. **Unified Pipeline**: Collector metrics flow through the same pipeline as application metrics

This allows you to monitor the collector's health and the semconv processor's effectiveness using the same tools as your application monitoring.

Access metrics at:
- Prometheus format: `http://localhost:8888/metrics`
- Test self-monitoring: `./test-self-monitoring.sh`

Example PromQL queries:
```promql
# Spans processed per second by rule
rate(otelcol_processor_semconv_spans_processed[1m])

# Cardinality reduction ratio (benchmark mode)
otelcol_processor_semconv_reduced_span_name_count / otelcol_processor_semconv_original_span_name_count

# Processing latency percentiles
histogram_quantile(0.99, otelcol_processor_semconv_processing_duration_bucket)
```

## Development

### Testing Changes

Build and test the processor:
```bash
cd processors/semconvprocessor
go test ./...
```

### Docker Development

The Docker image provides a production-ready deployment:

```bash
# Build image with specific tag
docker build -t otelcol-semconv:dev .

# Run with custom config
docker run -d \
  --name otelcol-test \
  -v $(pwd)/custom-config.yaml:/etc/otelcol/config.yaml:ro \
  -p 4317:4317 \
  otelcol-semconv:dev

# View logs
docker logs -f otelcol-test

# Access health check
curl http://localhost:13133/

# View metrics
curl http://localhost:8888/metrics
```

### Docker Compose Setup

The included `docker-compose.yaml` provides a complete testing environment:

```bash
# Start all services (collector + Jaeger)
docker-compose up -d

# View logs
docker-compose logs -f otelcol-semconv

# Stop services
docker-compose down

# Rebuild after changes
docker-compose build
docker-compose up -d
```

Access services:
- OTLP gRPC: `localhost:4317`
- OTLP HTTP: `localhost:4318`
- Metrics: `http://localhost:8888/metrics`
- Health: `http://localhost:13133/`
- Jaeger UI: `http://localhost:16686`

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