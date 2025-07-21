#!/bin/bash

echo "Testing collector self-monitoring setup..."

# Check if collector metrics endpoint is accessible
echo -n "Checking collector metrics endpoint... "
if curl -s http://localhost:8888/metrics > /dev/null 2>&1; then
    echo "✓ OK"
else
    echo "✗ Failed"
    exit 1
fi

# Check for semconv processor metrics
echo -n "Checking for semconv processor metrics... "
METRICS=$(curl -s http://localhost:8888/metrics)

if echo "$METRICS" | grep -q "otelcol_processor_semconv_spans_processed"; then
    echo "✓ Found spans_processed metric"
else
    echo "✗ Missing spans_processed metric"
fi

if echo "$METRICS" | grep -q "otelcol_processor_semconv_span_names_enforced"; then
    echo "✓ Found span_names_enforced metric"
else
    echo "✗ Missing span_names_enforced metric"
fi

if echo "$METRICS" | grep -q "otelcol_processor_semconv_processing_duration"; then
    echo "✓ Found processing_duration metric"
else
    echo "✗ Missing processing_duration metric"
fi

# Check for benchmark metrics if enabled
if echo "$METRICS" | grep -q "otelcol_processor_semconv_original_span_name_count"; then
    echo "✓ Found benchmark metrics (original_span_name_count)"
fi

if echo "$METRICS" | grep -q "otelcol_processor_semconv_reduced_span_name_count"; then
    echo "✓ Found benchmark metrics (reduced_span_name_count)"
fi

# Check for general collector metrics
echo -n "Checking for general collector metrics... "
if echo "$METRICS" | grep -q "otelcol_process_uptime"; then
    echo "✓ Found uptime metric"
else
    echo "✗ Missing uptime metric"
fi

echo ""
echo "Sample metrics:"
echo "$METRICS" | grep -E "(otelcol_processor_semconv|otelcol_process_uptime)" | head -10