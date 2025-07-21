# Semantic Convention Processor

The semconv processor enables processing and transformation of semantic conventions in OpenTelemetry data.

## Configuration

The following settings can be configured:

- `enabled` (default: false): Enable/disable the processor
- `mappings`: List of attribute mappings to apply

### Mapping Actions

- `rename`: Rename an attribute (removes the old attribute)
- `copy`: Copy an attribute value to a new attribute (keeps the old attribute)
- `move`: Move an attribute to a new name (alias for rename)

## Example

```yaml
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
```