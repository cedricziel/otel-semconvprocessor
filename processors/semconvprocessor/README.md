# Semantic Convention Processor

The semconv processor enforces semantic conventions to reduce cardinality in OpenTelemetry data using OTTL (OpenTelemetry Transformation Language) for flexible span name processing and operation name generation.

## Key Features

- **OTTL-based rule engine** for flexible span name processing
- **Dual processing modes**: enrich (add attributes only) or enforce (override span names)
- **Rule prioritization** with first-match-wins behavior
- **Custom OTTL functions** for common patterns (NormalizePath, ParseSQL, RemoveQueryParams)
- **Cardinality reduction metrics** to track effectiveness
- **Configurable operation name and type attributes**

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
      original_name_attribute: "span.name.original"
      rules:
        - id: "http_routes"
          priority: 100
          condition: 'attributes["http.method"] != nil and attributes["http.route"] != nil'
          operation_name: 'Concat([attributes["http.method"], attributes["http.route"]], " ")'
          operation_type: '"http"'
        - id: "database_queries"
          priority: 200
          condition: 'attributes["db.statement"] != nil'
          operation_name: 'ParseSQL(attributes["db.statement"])'
          operation_type: '"database"'
```

### Processing Modes

- **`enrich`**: Adds operation name and type as attributes, preserves original span names
- **`enforce`**: Replaces span names with operation names for cardinality reduction

### Rule Configuration

Each rule must specify:
- **`id`**: Unique identifier for the rule
- **`priority`**: Lower numbers = higher priority (processed first)
- **`condition`**: OTTL boolean expression to match spans
- **`operation_name`**: OTTL expression to generate the operation name
- **`operation_type`** (optional): OTTL expression for operation type

## OTTL Examples

### HTTP Route Normalization

```yaml
- id: "http_normalization"
  priority: 100
  condition: 'attributes["http.method"] != nil and attributes["url.path"] != nil'
  operation_name: 'Concat([attributes["http.method"], NormalizePath(attributes["url.path"])], " ")'
  operation_type: '"http"'
```

This transforms:
- `GET /users/12345/profile` → `GET /users/{id}/profile`
- `POST /api/v1/orders/550e8400-e29b-41d4-a716-446655440000` → `POST /api/v1/orders/{id}`

### Database Query Processing

```yaml
- id: "database_operations"
  priority: 200
  condition: 'attributes["db.statement"] != nil'
  operation_name: 'ParseSQL(attributes["db.statement"])'
  operation_type: 'attributes["db.system"]'
```

This transforms:
- `SELECT * FROM users WHERE id = ?` → `SELECT users`
- `INSERT INTO products (name, price) VALUES (?, ?)` → `INSERT products`

### Custom Business Logic

```yaml
- id: "service_endpoints"
  priority: 300
  condition: 'attributes["service.name"] == "user-service" and span.kind == SPAN_KIND_SERVER'
  operation_name: 'Concat([attributes["service.name"], attributes["rpc.method"]], "::")'
  operation_type: '"rpc"'
```

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
        # High-priority HTTP route normalization
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
        
        # Database operations
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