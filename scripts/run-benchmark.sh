#!/bin/bash

# Run benchmark for the semconv processor
# This script builds the collector, runs it with test data, and extracts metrics

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
FIXTURE_FILE="$PROJECT_ROOT/benchmark/otel-demo.log"
CONFIG_FILE="$PROJECT_ROOT/config.yaml"
BENCHMARK_FILE="$PROJECT_ROOT/BENCHMARK.md"
COLLECTOR_BINARY="$PROJECT_ROOT/otelcol-semconv/otelcol-semconv"
COLLECTOR_LOG="$PROJECT_ROOT/collector.log"
METRICS_OUTPUT="$PROJECT_ROOT/benchmark_metrics.txt"

# Function to print colored output
print_status() {
    echo -e "${BLUE}==>${NC} $1"
}

print_success() {
    echo -e "${GREEN}✓${NC} $1"
}

print_error() {
    echo -e "${RED}✗${NC} $1"
}

# Cleanup function
cleanup() {
    print_status "Cleaning up..."
    # Kill any running collector processes
    pkill -f "otelcol-semconv" 2>/dev/null || true
    
    # Also kill the specific PID if we have it
    if [ -n "$COLLECTOR_PID" ] && kill -0 "$COLLECTOR_PID" 2>/dev/null; then
        kill "$COLLECTOR_PID" 2>/dev/null || true
        wait "$COLLECTOR_PID" 2>/dev/null || true
    fi
    
    # Clean up temporary files
    rm -f "$COLLECTOR_LOG" "$METRICS_OUTPUT" || true
}

# Ensure cleanup runs on all exits
trap cleanup EXIT INT TERM

# Check prerequisites
print_status "Checking prerequisites..."

if [ ! -f "$FIXTURE_FILE" ]; then
    print_error "Fixture file not found: $FIXTURE_FILE"
    exit 1
fi

if ! command -v jq &> /dev/null; then
    print_error "jq is required but not installed. Please install jq."
    exit 1
fi

if ! command -v curl &> /dev/null; then
    print_error "curl is required but not installed. Please install curl."
    exit 1
fi

print_success "Prerequisites checked"

# Build the collector
print_status "Building the collector..."
cd "$PROJECT_ROOT"
if command -v ocb &> /dev/null; then
    ocb --config=builder-config.yaml
else
    echo "ocb (OpenTelemetry Collector Builder) is required but not installed."
    echo "Please install it with: go install go.opentelemetry.io/collector/cmd/builder@latest"
    echo "Then add it to your PATH as 'ocb'"
    exit 1
fi

if [ ! -f "$COLLECTOR_BINARY" ]; then
    print_error "Failed to build collector binary"
    exit 1
fi

print_success "Collector built successfully"

# Create benchmark directory and clean up old output
mkdir -p "$PROJECT_ROOT/benchmark"
rm -f "$PROJECT_ROOT/benchmark/output.log"

# Start the collector
print_status "Starting the collector with benchmark mode..."
"$COLLECTOR_BINARY" --config="$CONFIG_FILE" > "$COLLECTOR_LOG" 2>&1 &
COLLECTOR_PID=$!

# Wait for collector to start
print_status "Waiting for collector to start..."
for i in {1..30}; do
    if curl -s http://localhost:8888/metrics > /dev/null 2>&1; then
        print_success "Collector started"
        break
    fi
    if [ $i -eq 30 ]; then
        print_error "Collector failed to start. Check $COLLECTOR_LOG for details"
        tail -20 "$COLLECTOR_LOG"
        exit 1
    fi
    sleep 1
done

# Get initial metrics snapshot (for counters)
print_status "Getting initial metrics snapshot..."
INITIAL_SPANS_PROCESSED=$(curl -s http://localhost:8888/metrics | grep "otelcol_processor_semconv_spans_processed__spans__total" | grep -v "#" | awk '{print $2}' | head -1)
if [ -z "$INITIAL_SPANS_PROCESSED" ]; then
    INITIAL_SPANS_PROCESSED=0
fi

# Send test data
print_status "Sending test data to the collector..."

# Count total lines (each line is a separate OTLP request)
TOTAL_LINES=$(wc -l < "$FIXTURE_FILE" | tr -d ' ')
print_status "Processing $TOTAL_LINES OTLP requests from fixture file..."

# Send data line by line
LINE_NUM=0
while IFS= read -r line; do
    LINE_NUM=$((LINE_NUM + 1))
    if [ -n "$line" ]; then
        # Send each line as a separate request
        curl -X POST http://localhost:4318/v1/traces \
            -H "Content-Type: application/json" \
            -d "$line" \
            -o /dev/null -s
        
        # Progress indicator every 100 lines
        if [ $((LINE_NUM % 100)) -eq 0 ]; then
            echo -ne "\r  Sent $LINE_NUM/$TOTAL_LINES requests..."
        fi
    fi
done < "$FIXTURE_FILE"

echo -e "\r  Sent $LINE_NUM/$TOTAL_LINES requests.    "
print_success "All data sent"

# Give the collector a moment to start writing
sleep 2

# Wait for processing to complete by monitoring benchmark output
print_status "Waiting for processing to complete..."

# Monitor the benchmark output file for changes
BENCHMARK_OUTPUT="$PROJECT_ROOT/benchmark/output.log"
LAST_SIZE=0
STABLE_COUNT=0
STABLE_THRESHOLD=3  # Number of checks without changes to consider processing complete
MAX_WAIT=60  # Maximum seconds to wait
WAIT_COUNT=0

while [ $STABLE_COUNT -lt $STABLE_THRESHOLD ] && [ $WAIT_COUNT -lt $MAX_WAIT ]; do
    if [ -f "$BENCHMARK_OUTPUT" ]; then
        CURRENT_SIZE=$(stat -f%z "$BENCHMARK_OUTPUT" 2>/dev/null || stat -c%s "$BENCHMARK_OUTPUT" 2>/dev/null || echo 0)
        echo -ne "\r  Waiting... File size: $CURRENT_SIZE bytes (stable for $STABLE_COUNT checks)"
        if [ "$CURRENT_SIZE" -gt 0 ]; then
            if [ "$CURRENT_SIZE" -eq "$LAST_SIZE" ]; then
                STABLE_COUNT=$((STABLE_COUNT + 1))
            else
                STABLE_COUNT=0
                LAST_SIZE=$CURRENT_SIZE
            fi
        fi
    else
        echo -ne "\r  Waiting... Output file not found yet ($WAIT_COUNT/$MAX_WAIT sec)"
    fi
    WAIT_COUNT=$((WAIT_COUNT + 2))
    sleep 2
done
echo ""  # New line after progress indicator

if [ $WAIT_COUNT -ge $MAX_WAIT ]; then
    print_error "Timeout waiting for processing to complete"
fi

print_success "Processing completed"

# Extract metrics
print_status "Extracting metrics..."
"$SCRIPT_DIR/monitor-metrics.sh" > "$METRICS_OUTPUT"

# Parse metrics
print_status "Parsing benchmark results..."

# Extract key metrics
TOTAL_SPANS=$(curl -s http://localhost:8888/metrics | grep "otelcol_processor_semconv_spans_processed__spans__total" | grep -v "#" | awk '{print $2}' | head -1)
SPANS_PROCESSED=$((TOTAL_SPANS - INITIAL_SPANS_PROCESSED))

ORIGINAL_COUNT=$(curl -s http://localhost:8888/metrics | grep "otelcol_processor_semconv_original_span_name_count__names_{" | awk '{print $2}' | head -1)
REDUCED_COUNT=$(curl -s http://localhost:8888/metrics | grep "otelcol_processor_semconv_reduced_span_name_count__names_{" | awk '{print $2}' | head -1)

# Get cumulative unique counts
UNIQUE_SPANS_TOTAL=$(curl -s http://localhost:8888/metrics | grep "otelcol_processor_semconv_unique_span_names__names__total{" | awk '{print $2}' | head -1)
UNIQUE_OPS_TOTAL=$(curl -s http://localhost:8888/metrics | grep "otelcol_processor_semconv_unique_operation_names__names__total{" | awk '{print $2}' | head -1)

# Get processing duration metrics
AVG_DURATION=$(curl -s http://localhost:8888/metrics | grep "otelcol_processor_semconv_processing_duration_milliseconds_sum" | awk '{print $2}' | head -1)
DURATION_COUNT=$(curl -s http://localhost:8888/metrics | grep "otelcol_processor_semconv_processing_duration_milliseconds_count" | awk '{print $2}' | head -1)

if [ -n "$AVG_DURATION" ] && [ -n "$DURATION_COUNT" ] && [ "$DURATION_COUNT" != "0" ]; then
    AVG_DURATION=$(echo "scale=3; $AVG_DURATION / $DURATION_COUNT" | bc)
else
    AVG_DURATION="N/A"
fi

# Calculate reduction percentage
if [ -n "$ORIGINAL_COUNT" ] && [ -n "$REDUCED_COUNT" ] && [ "$ORIGINAL_COUNT" != "0" ]; then
    REDUCTION_PCT=$(echo "scale=2; (($ORIGINAL_COUNT - $REDUCED_COUNT) * 100) / $ORIGINAL_COUNT" | bc)
else
    REDUCTION_PCT="N/A"
fi

# Calculate throughput
if [ -n "$AVG_DURATION" ] && [ "$AVG_DURATION" != "N/A" ] && [ "$AVG_DURATION" != "0" ]; then
    THROUGHPUT=$(echo "scale=0; 1000 / $AVG_DURATION" | bc)
else
    THROUGHPUT="N/A"
fi

# Get rule effectiveness
print_status "Analyzing rule effectiveness..."
RULES_DATA=$(curl -s http://localhost:8888/metrics | grep "otelcol_processor_semconv_span_names_enforced__operations__total" | grep -v "#" | while read line; do
    RULE_ID=$(echo "$line" | sed -n 's/.*rule_id="\([^"]*\)".*/\1/p')
    COUNT=$(echo "$line" | awk '{print $2}')
    if [ -n "$RULE_ID" ] && [ -n "$COUNT" ] && [ "$COUNT" != "0" ]; then
        echo "$RULE_ID|$COUNT"
    fi
done | sort -t'|' -k2 -nr)

# Display results
echo ""
print_success "Benchmark completed!"
echo ""
echo -e "${GREEN}=== Benchmark Results ===${NC}"
echo ""
echo "Cardinality Reduction:"
echo "  Original Unique Span Names: ${ORIGINAL_COUNT:-N/A}"
echo "  Reduced Unique Operation Names: ${REDUCED_COUNT:-N/A}"
echo "  Cardinality Reduction: ${REDUCTION_PCT}%"
echo "  Total Spans Processed: ${SPANS_PROCESSED:-N/A}"
echo ""
echo "Processing Performance:"
echo "  Average Processing Duration: ${AVG_DURATION}ms"
echo "  Processing Throughput: ${THROUGHPUT} spans/sec"
echo ""
echo "Top Rules by Effectiveness:"
echo "$RULES_DATA" | head -10 | while IFS='|' read -r rule count; do
    if [ -n "$rule" ] && [ -n "$count" ]; then
        printf "  %-30s: %s spans\n" "$rule" "$count"
    fi
done

# Update BENCHMARK.md
print_status "Updating BENCHMARK.md..."
TIMESTAMP=$(date +"%Y-%m-%d %H:%M:%S %Z")

# Create a temporary file with updated results
TEMP_FILE=$(mktemp)

# Prepare rules data in a safer format (escape newlines)
RULES_DATA_ESCAPED=$(echo "$RULES_DATA" | tr '\n' '\034')

# Read the benchmark file and update the results section
awk -v timestamp="$TIMESTAMP" \
    -v original="${ORIGINAL_COUNT:-N/A}" \
    -v reduced="${REDUCED_COUNT:-N/A}" \
    -v reduction="${REDUCTION_PCT:-N/A}%" \
    -v total="${SPANS_PROCESSED:-0}" \
    -v avg_duration="${AVG_DURATION:-N/A}ms" \
    -v throughput="${THROUGHPUT:-N/A} spans/sec" \
    -v rules_data="$RULES_DATA_ESCAPED" \
    '
BEGIN {
    in_cardinality = 0
    in_performance = 0
    in_rules = 0
    printed_rules = 0
}
/^## Latest Results/ {
    print
    print ""
    print "*Last updated: " timestamp "*"
    print ""
    # Skip any existing content until we hit the next section
    while (getline && !/^###/) {}
}
/^### Cardinality Reduction/ {
    in_cardinality = 1
    in_performance = 0
    in_rules = 0
    print
    print ""  # Add empty line after the heading
    next
}
/^### Processing Performance/ {
    in_cardinality = 0
    in_performance = 1
    in_rules = 0
    print
    print ""  # Add empty line after the heading
    next
}
/^### Rule Effectiveness/ {
    in_cardinality = 0
    in_performance = 0
    in_rules = 1
    print
    print ""
    next
}
/^### / && (in_cardinality || in_performance || in_rules) {
    in_cardinality = 0
    in_performance = 0
    in_rules = 0
    print ""  # Ensure empty line before next section
}
in_cardinality == 1 {
    if (/^$/) {
        # Print table without extra empty line (one is already there)
        print "| Metric | Value | Description |"
        print "|--------|-------|-------------|"
        print "| **Original Unique Span Names** | " original " | Number of unique span names before processing |"
        print "| **Reduced Unique Operation Names** | " reduced " | Number of unique operation names after processing |"
        print "| **Cardinality Reduction** | " reduction " | Percentage reduction in unique values |"
        print "| **Total Spans Processed** | " total " | Total number of spans in the benchmark |"
        print ""
        in_cardinality = 2
        # Skip any existing table
        while (getline && /^\|/) {}
    }
}
in_performance == 1 {
    if (/^$/) {
        # Print table without extra empty line (one is already there)
        print "| Metric | Value | Description |"
        print "|--------|-------|-------------|"
        print "| **Average Processing Duration** | " avg_duration " | Average time to process a batch (ms) |"
        print "| **Processing Throughput** | " throughput " | Spans processed per second |"
        print ""
        in_performance = 2
        # Skip any existing table
        while (getline && /^\|/) {}
    }
}
in_rules && /^\| Rule ID/ && !printed_rules {
    print
    getline  # Skip separator line
    print
    # Print rules data (unescape the special character)
    gsub(/\034/, "\n", rules_data)
    # Split into lines and print in order (already sorted)
    n = split(rules_data, lines, "\n")
    count = 0
    for (i = 1; i <= n && count < 10; i++) {
        if (lines[i] != "") {
            split(lines[i], parts, "|")
            if (length(parts) == 2 && parts[2] > 0) {
                pct = sprintf("%.1f", (parts[2] / total) * 100)
                printf "| %s | %s | %s%% | - |\n", parts[1], parts[2], pct
                count++
            }
        }
    }
    printed_rules = 1
    # Skip remaining table rows
    while (getline && /^\|/) {}
    print ""  # Add empty line after rules table
    next
}
# Ensure empty line before ### Version Comparison
/^### Version Comparison/ {
    print ""
    print
    getline  # Skip empty line
    print ""
    getline  # Skip table header
    print
    getline  # Skip separator
    print
    # Update the Current row
    getline
    if (/^\| Current/) {
        print "| Current | " original " | " reduced " | " reduction " | " avg_duration " |"
        # Skip any remaining rows
        while (getline && /^\|/) {}
    }
    next
}
!in_cardinality && !in_performance && !in_rules {
    print
}
' "$BENCHMARK_FILE" > "$TEMP_FILE"

# Move the temporary file to the benchmark file
mv "$TEMP_FILE" "$BENCHMARK_FILE"

print_success "BENCHMARK.md updated with latest results"

# Show collector logs if there were errors
if grep -i "error\|warn" "$COLLECTOR_LOG" > /dev/null 2>&1; then
    echo ""
    print_status "Collector warnings/errors detected:"
    grep -i "error\|warn" "$COLLECTOR_LOG" | tail -10
fi

echo ""
print_success "Benchmark complete! Results saved to $BENCHMARK_FILE"