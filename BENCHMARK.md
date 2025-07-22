# Semconv Processor Benchmark Results

This document contains benchmark results for the OpenTelemetry Semantic Convention Processor, demonstrating its effectiveness in reducing span name cardinality while maintaining observability.

## Overview

The semconv processor is designed to normalize span names according to semantic conventions, reducing cardinality explosion in telemetry systems. This benchmark measures:

1. **Cardinality Reduction**: How effectively the processor reduces unique span names
2. **Processing Performance**: How fast the processor can handle spans
3. **Rule Effectiveness**: Which rules contribute most to cardinality reduction

## Methodology

### Test Data
- **Source**: Real-world OpenTelemetry spans from the OpenTelemetry Demo application
- **Collection Method**: Traffic capture from vanilla OpenTelemetry Demo deployment
- **Size**: ~4.4MB of JSON-formatted span data
- **Span Count**: Varies per run (typically 1000-2000 spans)
- **Services**: Mix of frontend, backend, database, messaging, and proxy services

### Configuration
- **Mode**: Enforce mode (span names are rewritten)
- **Benchmark**: Enabled (tracks cardinality metrics)
- **Rules**: Full set of semantic convention rules including:
  - HTTP server/client normalization
  - Database query parsing
  - gRPC method standardization
  - Messaging operation normalization
  - GraphQL operation handling

### Process
1. Build the collector with benchmark mode enabled
2. Start the collector with the test configuration
3. Feed fixture data through the OTLP receiver
4. Wait for processing completion
5. Extract metrics from Prometheus endpoint
6. Calculate and format results

## Latest Results

*Last updated: 2025-07-22 09:39:06 CEST*

### Cardinality Reduction

| Metric | Value | Description |
|--------|-------|-------------|
| **Original Unique Span Names** | 109 | Number of unique span names before processing |
| **Reduced Unique Operation Names** | 40 | Number of unique operation names after processing |
| **Cardinality Reduction** | 63.30% | Percentage reduction in unique values |
| **Total Spans Processed** | 4154 | Total number of spans in the benchmark |

### Processing Performance

| Metric | Value | Description |
|--------|-------|-------------|
| **Average Processing Duration** | .044ms | Average time to process a batch (ms) |
| **Processing Throughput** | 22727 spans/sec | Spans processed per second |

### Rule Effectiveness

| Rule ID | Spans Matched | Percentage | Description |
|---------|---------------|------------|-------------|
| http_server_method_only | 922 | 22.2% | - |
| grpc_client_operations | 614 | 14.8% | - |
| grpc_server_operations | 457 | 11.0% | - |
| http_client_method_only | 398 | 9.6% | - |
| http_server_routes | 284 | 6.8% | - |
| database_queries | 272 | 6.5% | - |
| messaging_system | 35 | 0.8% | - |
| messaging_producer | 17 | 0.4% | - |
| messaging_consumer | 17 | 0.4% | - |
| database_queries | 17 | 0.4% | - |

### Version Comparison

| Version | Original Names | Reduced Names | Reduction % | Avg Duration (ms) |
|---------|----------------|---------------|-------------|-------------------|
| Current | 109 | 40 | 63.30% | .044ms |
To run the benchmark yourself:

```bash
# Run the benchmark script
./scripts/run-benchmark.sh

# View real-time metrics during the run
./scripts/monitor-metrics.sh --watch
```

The benchmark script will:
1. Build a fresh collector binary
2. Start the collector with benchmark configuration
3. Process the test data
4. Extract and format the results
5. Update this file with the latest metrics

## Interpreting Results

### Good Cardinality Reduction
- **60%+ reduction**: Excellent - significant cardinality improvement
- **40-60% reduction**: Good - meaningful reduction in storage/query costs
- **20-40% reduction**: Moderate - some benefit, consider adding more rules
- **<20% reduction**: Low - may need to review rule effectiveness

### Performance Targets
- **<1ms average duration**: Excellent for high-volume environments
- **1-5ms average duration**: Good for most use cases
- **>5ms average duration**: May need optimization for high-volume scenarios

## Notes

- Results may vary based on the diversity of span names in your environment
- The fixture data represents a typical microservices application
- Custom rules can be added to improve reduction for specific use cases
- Performance is measured on a single-threaded collector instance
