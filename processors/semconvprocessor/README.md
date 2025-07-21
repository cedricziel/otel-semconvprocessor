# Semantic Convention Processor

The semconv processor enforces semantic conventions to reduce cardinality in OpenTelemetry data. It focuses on normalizing span names to prevent cardinality explosion while maintaining meaningful observability.

## Configuration

The following settings can be configured:

- `enabled` (default: false): Enable/disable the processor
- `mappings`: List of attribute mappings to apply

### Mapping Actions

- `rename`: Rename an attribute (removes the old attribute)
- `copy`: Copy an attribute value to a new attribute (keeps the old attribute)
- `move`: Move an attribute to a new name (alias for rename)

## Why This Processor?

High-cardinality span names can cause:
- Increased storage costs
- Slower query performance
- Difficulty in creating meaningful dashboards and alerts
- Memory pressure on observability backends

This processor enforces OpenTelemetry semantic conventions to ensure span names remain low-cardinality while preserving essential information in attributes.

## Example

```yaml
processors:
  semconv:
    enabled: true
    # Migrate to newer semantic conventions
    mappings:
      - from: "http.method"
        to: "http.request.method"
        action: "rename"
    # Enforce low-cardinality span names
    span_name_rules:
      enabled: true
      http:
        use_url_template: true
        remove_query_params: true
        remove_path_params: true
```

This configuration ensures HTTP spans like `GET /users/12345?include=profile` become `GET /users/{id}` while preserving the original details in attributes.

## Telemetry Metrics

The processor exports the following metrics to monitor its performance and behavior:

### Counter Metrics

- `processor_semconv_attributes_renamed` - Number of attributes renamed by the processor
- `processor_semconv_attributes_copied` - Number of attributes copied by the processor
- `processor_semconv_attributes_moved` - Number of attributes moved by the processor
- `processor_semconv_spans_processed` - Number of spans processed (with `signal_type` attribute)
- `processor_semconv_span_names_enforced` - Number of span names changed to match semantic conventions (with `convention_type` attribute: http, database, messaging, custom)
- `processor_semconv_errors` - Number of errors encountered during processing (with `error_type` attribute)

### Histogram Metrics

- `processor_semconv_processing_duration` - Time taken to process a batch of telemetry in milliseconds (with `signal_type` attribute)

These metrics are automatically exported through the collector's configured telemetry pipeline and can be used to:
- Monitor the impact of semantic convention enforcement
- Track processing performance
- Identify potential configuration issues
- Understand which types of transformations are most common