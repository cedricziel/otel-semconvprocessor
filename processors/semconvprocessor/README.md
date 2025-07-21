# Semantic Convention Processor

The semconv processor enforces semantic conventions to reduce cardinality in OpenTelemetry data using OTTL (OpenTelemetry Transformation Language) for flexible span name processing and operation name generation.

## Key Features

- **OTTL-based rule engine** for flexible span name processing
- **Dual processing modes**: enrich (add attributes only) or enforce (override span names)
- **Rule prioritization** with first-match-wins behavior
- **Custom OTTL functions** for common patterns (NormalizePath, ParseSQL, RemoveQueryParams)
- **Cardinality reduction metrics** to track effectiveness
- **Configurable operation name and type attributes**
- **Respects existing attributes**: Skips processing if `operation.name` already exists, doesn't override existing `operation.type`

## Configuration

### Basic Structure

```yaml
processors:
  semconv:
    enabled: true
    benchmark: true  # Enable cardinality tracking
    span_processing:
      enabled: true
      mode: "enforce"  # or "enrich"
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
        - id: "http_client_requests"
          priority: 150
          span_kind: ["client"]  # Only matches client spans
          condition: 'attributes["http.method"] != nil and attributes["http.url"] != nil'
          operation_name: 'Concat([attributes["http.method"], RemoveQueryParams(attributes["http.url"])], " ")'
          operation_type: '"http_client"'
        - id: "database_queries"
          priority: 200
          # No span_kind specified - matches all span kinds
          condition: 'attributes["db.statement"] != nil'
          operation_name: 'ParseSQL(attributes["db.statement"])'
          operation_type: '"database"'
```

### Processing Modes

- **`enrich`**: Adds operation name and type as attributes, preserves original span names
- **`enforce`**: Replaces span names with operation names for cardinality reduction

### Attribute Handling

The processor respects existing attributes:
- **Skips processing entirely** if `operation.name` attribute already exists on the span
- **Does not override** existing `operation.type` attributes - only sets if not present
- This allows upstream processors or instrumentation to set these attributes and have them preserved

### Rule Configuration

Each rule must specify:
- **`id`**: Unique identifier for the rule
- **`priority`**: Lower numbers = higher priority (processed first)
- **`condition`**: OTTL boolean expression to match spans
- **`operation_name`**: OTTL expression to generate the operation name
- **`operation_type`** (optional): OTTL expression for operation type
- **`span_kind`** (optional): List of span kinds to match (`server`, `client`, `producer`, `consumer`, `internal`)

## OTTL Examples

### HTTP Route Normalization with Span Kind Filtering

```yaml
# Server-side HTTP spans
- id: "http_server_routes"
  priority: 100
  span_kind: ["server"]  # Only matches server spans
  condition: 'attributes["http.method"] != nil and attributes["http.route"] != nil'
  operation_name: 'Concat([attributes["http.method"], attributes["http.route"]], " ")'
  operation_type: '"http"'

# Client-side HTTP spans
- id: "http_client_requests"
  priority: 150
  span_kind: ["client"]  # Only matches client spans
  condition: 'attributes["http.method"] != nil and attributes["http.url"] != nil'
  operation_name: 'Concat([attributes["http.method"], RemoveQueryParams(attributes["http.url"])], " ")'
  operation_type: '"http_client"'

# Fallback for any HTTP span (no span_kind restriction)
- id: "http_generic"
  priority: 200
  condition: 'attributes["http.method"] != nil and attributes["url.path"] != nil'
  operation_name: 'Concat([attributes["http.method"], NormalizePath(attributes["url.path"])], " ")'
  operation_type: '"http"'
```

This transforms:
- Server span: `GET /users/12345/profile` → `GET /users/{id}/profile`
- Client span: `POST https://api.example.com/orders?id=123` → `POST https://api.example.com/orders`

### Database Query Processing

```yaml
# Database client operations
- id: "database_client_operations"
  priority: 200
  span_kind: ["client"]  # Database operations are typically client spans
  condition: 'attributes["db.statement"] != nil'
  operation_name: 'ParseSQL(attributes["db.statement"])'
  operation_type: 'attributes["db.system"]'

# Database internal operations (e.g., triggers, stored procedures)
- id: "database_internal_operations"
  priority: 250
  span_kind: ["internal"]
  condition: 'attributes["db.operation"] != nil and attributes["db.name"] != nil'
  operation_name: 'Concat([attributes["db.operation"], attributes["db.name"]], " ")'
  operation_type: '"db_internal"'
```

This transforms:
- Client span: `SELECT * FROM users WHERE id = ?` → `SELECT users`
- Internal span: `EXEC stored_proc` → `EXEC mydb`

### Messaging Operations with Span Kind

```yaml
# Producer spans
- id: "kafka_producer"
  priority: 300
  span_kind: ["producer"]
  condition: 'attributes["messaging.system"] == "kafka"'
  operation_name: 'Concat(["kafka.produce", attributes["messaging.destination.name"]], ":")'
  operation_type: '"messaging"'

# Consumer spans
- id: "kafka_consumer"
  priority: 350
  span_kind: ["consumer"]
  condition: 'attributes["messaging.system"] == "kafka"'
  operation_name: 'Concat(["kafka.consume", attributes["messaging.destination.name"]], ":")'
  operation_type: '"messaging"'
```

### Span Kind Filtering

The `span_kind` property allows rules to target specific types of spans:

```yaml
# Multiple span kinds - matches either server OR internal spans
- id: "internal_operations"
  priority: 500
  span_kind: ["server", "internal"]
  condition: 'attributes["internal.operation"] != nil'
  operation_name: 'attributes["internal.operation"]'
  operation_type: '"internal"'

# No span_kind specified - matches ALL span kinds
- id: "catch_all"
  priority: 1000
  condition: 'true'
  operation_name: '"unknown_operation"'
  operation_type: '"unknown"'
```

Valid span kind values:
- `server`: Synchronous request handler
- `client`: Synchronous outbound request
- `producer`: Asynchronous message/event producer
- `consumer`: Asynchronous message/event consumer
- `internal`: Internal operation not at a system boundary

## Custom OTTL Functions

The processor provides additional OTTL functions:

### NormalizePath(path)

Normalizes URL paths by replacing identifiers with placeholders:

```ottl
NormalizePath("/users/12345/posts/67890")  # → "/users/{id}/posts/{id}"
NormalizePath("/api/orders/550e8400-e29b-41d4-a716-446655440000")  # → "/api/orders/{id}"
```

Handles:
- UUIDs → `{id}`
- Numeric IDs → `{id}`
- MongoDB ObjectIDs → `{id}`
- Query parameters (removes them)

### ParseSQL(statement)

Extracts operation and table from SQL statements:

```ottl
ParseSQL("SELECT * FROM users WHERE id = ?")  # → "SELECT users"
ParseSQL("INSERT INTO products (name) VALUES (?)")  # → "INSERT products"
ParseSQL("UPDATE customers SET email = ? WHERE id = ?")  # → "UPDATE customers"
```

Handles schema prefixes and quoted identifiers automatically.

### RemoveQueryParams(url)

Removes query parameters from URLs:

```ottl
RemoveQueryParams("/search?q=test&limit=10")  # → "/search"
```

## Complete Example

```yaml
processors:
  semconv:
    enabled: true
    benchmark: true
    span_processing:
      enabled: true
      mode: "enforce"
      preserve_original_name: true
      rules:
        # HTTP server spans - route-based
        - id: "http_server_routes"
          priority: 100
          span_kind: ["server"]
          condition: 'attributes["http.method"] != nil and attributes["http.route"] != nil'
          operation_name: 'Concat([attributes["http.method"], attributes["http.route"]], " ")'
          operation_type: '"http"'
        
        # HTTP client spans - URL-based
        - id: "http_client_requests"
          priority: 150
          span_kind: ["client"]
          condition: 'attributes["http.method"] != nil and attributes["http.url"] != nil'
          operation_name: 'Concat([attributes["http.method"], RemoveQueryParams(attributes["http.url"])], " ")'
          operation_type: '"http_client"'
        
        # HTTP path normalization (any span kind)
        - id: "http_paths"
          priority: 200
          condition: 'attributes["http.method"] != nil and attributes["url.path"] != nil'
          operation_name: 'Concat([attributes["http.method"], NormalizePath(attributes["url.path"])], " ")'
          operation_type: '"http"'
        
        # Database client operations
        - id: "database_queries"
          priority: 300
          span_kind: ["client"]
          condition: 'attributes["db.statement"] != nil'
          operation_name: 'ParseSQL(attributes["db.statement"])'
          operation_type: 'attributes["db.system"]'
        
        # Messaging producer operations
        - id: "messaging_producer"
          priority: 400
          span_kind: ["producer"]
          condition: 'attributes["messaging.operation"] == "publish"'
          operation_name: 'Concat(["publish", attributes["messaging.destination.name"]], " ")'
          operation_type: '"messaging"'
        
        # Messaging consumer operations
        - id: "messaging_consumer"
          priority: 450
          span_kind: ["consumer"]
          condition: 'attributes["messaging.operation"] == "process"'
          operation_name: 'Concat(["process", attributes["messaging.destination.name"]], " ")'
          operation_type: '"messaging"'

service:
  pipelines:
    traces:
      processors: [semconv]
```

## Telemetry Metrics

The processor exports metrics to monitor cardinality reduction effectiveness:

### Counter Metrics

- `otelcol_processor_semconv_spans_processed` - Total spans processed
- `otelcol_processor_semconv_span_names_enforced` - Span names changed (with `rule_id` attribute)
- `otelcol_processor_semconv_errors` - Processing errors

### Histogram Metrics

- `otelcol_processor_semconv_processing_duration` - Processing time in milliseconds

### Benchmark Metrics (when `benchmark: true`)

- `otelcol_processor_semconv_original_span_name_count` - Unique span names before processing
- `otelcol_processor_semconv_reduced_span_name_count` - Unique span names after processing

Use these metrics to:
- Track cardinality reduction effectiveness
- Monitor processing performance
- Identify configuration issues
- Understand rule match patterns

## Migration from Attribute Mapping

This processor previously supported attribute mapping functionality. The new OTTL-based approach provides much more flexibility and power. If you were using the old attribute mapping features, they have been removed in favor of the OTTL rule-based system which can accomplish the same goals and much more.