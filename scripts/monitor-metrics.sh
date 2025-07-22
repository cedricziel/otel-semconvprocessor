#!/bin/bash

# Monitor semconv processor metrics
# This script scrapes the collector's Prometheus endpoint and extracts
# key metrics related to the semconv processor's performance

set -e

# Configuration
METRICS_ENDPOINT="${METRICS_ENDPOINT:-localhost:8888/metrics}"
INTERVAL="${INTERVAL:-5}" # seconds between scrapes
WATCH_MODE="${WATCH_MODE:-false}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to fetch and parse metrics
fetch_metrics() {
    curl -s "http://${METRICS_ENDPOINT}" 2>/dev/null || {
        echo -e "${RED}Error: Could not connect to metrics endpoint at ${METRICS_ENDPOINT}${NC}"
        echo "Make sure the collector is running and the metrics endpoint is accessible."
        exit 1
    }
}

# Function to extract specific metric value
get_metric_value() {
    local metric_name="$1"
    local labels="$2"
    local metrics="$3"
    
    if [ -z "$labels" ]; then
        echo "$metrics" | grep "^${metric_name} " | awk '{print $2}' | tail -1
    else
        echo "$metrics" | grep "^${metric_name}{" | grep "$labels" | awk '{print $2}' | tail -1
    fi
}

# Function to calculate rate (difference between two values)
calculate_rate() {
    local current="$1"
    local previous="$2"
    local interval="$3"
    
    if [ -z "$previous" ] || [ -z "$current" ]; then
        echo "0"
    else
        echo "scale=2; ($current - $previous) / $interval" | bc 2>/dev/null || echo "0"
    fi
}

# Function to format large numbers
format_number() {
    local num="$1"
    # Remove any scientific notation or special characters
    num=$(echo "$num" | sed 's/[^0-9.]//g')
    
    if [ -z "$num" ]; then
        echo "0"
        return
    fi
    
    # Check if it's a valid number
    if ! [[ "$num" =~ ^[0-9]+\.?[0-9]*$ ]]; then
        echo "0"
        return
    fi
    
    # Use awk for comparison to avoid bc issues
    if (( $(echo "$num >= 1000000" | awk '{print ($1 >= 1000000)}') )); then
        echo "$(echo "$num" | awk '{printf "%.2fM", $1/1000000}')"
    elif (( $(echo "$num >= 1000" | awk '{print ($1 >= 1000)}') )); then
        echo "$(echo "$num" | awk '{printf "%.2fK", $1/1000}')"
    else
        echo "$num"
    fi
}

# Function to display metrics summary
display_metrics() {
    local metrics="$1"
    
    clear
    echo -e "${BLUE}=== OpenTelemetry Collector Metrics ===${NC}"
    echo -e "Endpoint: ${METRICS_ENDPOINT}"
    echo -e "Time: $(date)"
    echo ""
    
    # Collector health
    echo -e "${GREEN}=== Collector Health ===${NC}"
    local uptime=$(echo "$metrics" | grep "^otelcol_process_uptime_seconds_total{" | awk '{print $2}' | head -1)
    if [ -n "$uptime" ]; then
        echo -e "Uptime: $(echo "scale=0; $uptime/60" | bc) minutes"
    fi
    
    local cpu_usage=$(echo "$metrics" | grep "^otelcol_process_cpu_seconds_total{" | awk '{print $2}' | head -1)
    if [ -z "$cpu_usage" ]; then
        cpu_usage="0"
    fi
    echo -e "CPU Time: ${cpu_usage}s"
    
    local mem_usage=$(echo "$metrics" | grep "^otelcol_process_memory_rss_bytes{" | awk '{print $2}' | head -1)
    if [ -n "$mem_usage" ]; then
        echo -e "Memory RSS: $(format_number $mem_usage) bytes"
    fi
    echo ""
    
    # Semconv processor metrics
    echo -e "${YELLOW}=== Semconv Processor Metrics ===${NC}"
    
    # Spans processed
    local spans_processed=$(get_metric_value "otelcol_processor_semconv_spans_processed__spans__total" "signal_type=\"traces\"" "$metrics")
    echo -e "Total Spans Processed: $(format_number ${spans_processed:-0})"
    
    # Span names enforced by rule
    echo -e "\nSpan Names Enforced by Rule:"
    
    # Get current mode from the config
    local current_mode=$(echo "$metrics" | grep "otelcol_processor_semconv_span_names_enforced__operations__total{" | grep -o 'mode="[^"]*"' | head -1 | cut -d'"' -f2)
    
    if [ -n "$current_mode" ]; then
        echo -e "Current Mode: ${YELLOW}${current_mode}${NC}"
        
        # Show metrics for current mode
        echo -e "\n${GREEN}Actual (mode=${current_mode}):${NC}"
        local rules=$(echo "$metrics" | grep "otelcol_processor_semconv_span_names_enforced__operations__total{" | grep "mode=\"${current_mode}\"" | sed -n 's/.*rule_id="\([^"]*\)".*/\1/p' | sort -u)
        
        if [ -n "$rules" ]; then
            for rule in $rules; do
                local count=$(echo "$metrics" | grep "otelcol_processor_semconv_span_names_enforced__operations__total" | grep "rule_id=\"$rule\"" | grep "mode=\"${current_mode}\"" | awk '{print $2}' | head -1)
                printf "  %-30s: %s\n" "$rule" "$(format_number ${count:-0})"
            done
        else
            echo "  No rules have matched yet"
        fi
        
        # Show what would be enforced in the other mode
        local other_mode="enforce"
        if [ "$current_mode" == "enforce" ]; then
            other_mode="enrich"
        fi
        
        echo -e "\n${BLUE}Would be enforced (mode=${other_mode}):${NC}"
        local other_rules=$(echo "$metrics" | grep "otelcol_processor_semconv_span_names_enforced__operations__total{" | grep "mode=\"${other_mode}\"" | sed -n 's/.*rule_id="\([^"]*\)".*/\1/p' | sort -u)
        
        if [ -n "$other_rules" ]; then
            for rule in $other_rules; do
                local count=$(echo "$metrics" | grep "otelcol_processor_semconv_span_names_enforced__operations__total" | grep "rule_id=\"$rule\"" | grep "mode=\"${other_mode}\"" | awk '{print $2}' | head -1)
                printf "  %-30s: %s\n" "$rule" "$(format_number ${count:-0})"
            done
        else
            echo "  No data for ${other_mode} mode"
        fi
    else
        # Fallback to old behavior if no mode attribute
        local rules=$(echo "$metrics" | grep "otelcol_processor_semconv_span_names_enforced__operations__total{" | sed -n 's/.*rule_id="\([^"]*\)".*/\1/p' | sort -u)
        
        if [ -n "$rules" ]; then
            for rule in $rules; do
                local count=$(get_metric_value "otelcol_processor_semconv_span_names_enforced__operations__total" "rule_id=\"$rule\"" "$metrics")
                printf "  %-30s: %s\n" "$rule" "$(format_number ${count:-0})"
            done
        else
            echo "  No rules have matched yet"
        fi
    fi
    
    # Cardinality metrics
    echo -e "\n${GREEN}=== Cardinality Reduction ===${NC}"
    local original_count=$(echo "$metrics" | grep "^otelcol_processor_semconv_original_span_name_count__names_{" | awk '{print $2}' | head -1)
    local reduced_count=$(echo "$metrics" | grep "^otelcol_processor_semconv_reduced_span_name_count__names_{" | awk '{print $2}' | head -1)
    
    if [ -n "$original_count" ] && [ -n "$reduced_count" ] && [ "$original_count" != "0" ] && [ "$original_count" != "" ]; then
        local reduction_pct=$(echo "scale=2; (($original_count - $reduced_count) * 100) / $original_count" | bc)
        echo -e "Original Unique Span Names: ${original_count} (current in memory)"
        echo -e "Reduced Unique Span Names: ${reduced_count} (current in memory)"
        echo -e "Cardinality Reduction: ${reduction_pct}%"
        
        # Show cumulative unique discoveries (counter metrics)
        local unique_spans_total=$(echo "$metrics" | grep "^otelcol_processor_semconv_unique_span_names__names__total{" | awk '{print $2}' | head -1)
        local unique_operations_total=$(echo "$metrics" | grep "^otelcol_processor_semconv_unique_operation_names__names__total{" | awk '{print $2}' | head -1)
        
        if [ -n "$unique_spans_total" ] || [ -n "$unique_operations_total" ]; then
            echo -e "\n${BLUE}Cumulative Unique Discoveries:${NC}"
            if [ -n "$unique_spans_total" ]; then
                echo -e "Total Unique Span Names Seen: $(format_number ${unique_spans_total})"
            fi
            if [ -n "$unique_operations_total" ]; then
                echo -e "Total Unique Operations Generated: $(format_number ${unique_operations_total})"
            fi
            
            # Calculate cumulative reduction percentage
            if [ -n "$unique_spans_total" ] && [ -n "$unique_operations_total" ] && [ "$unique_spans_total" != "0" ]; then
                local cumulative_reduction_pct=$(echo "scale=2; (($unique_spans_total - $unique_operations_total) * 100) / $unique_spans_total" | bc)
                echo -e "Cumulative Cardinality Reduction: ${cumulative_reduction_pct}%"
            fi
        fi
        
        # Show percentage of spans being normalized
        local enforced_total=0
        local enforced_counts=$(echo "$metrics" | grep "otelcol_processor_semconv_span_names_enforced__operations__total{" | awk '{print $2}')
        for count in $enforced_counts; do
            # Remove any non-numeric characters
            count=$(echo "$count" | sed 's/[^0-9.]//g')
            if [ -n "$count" ]; then
                enforced_total=$(echo "$enforced_total + $count" | awk '{print $1 + $2}')
            fi
        done
        
        if [ -n "$spans_processed" ] && [ "$spans_processed" != "0" ]; then
            # Extract just the numeric value from spans_processed
            local spans_processed_num=$(echo "$spans_processed" | sed 's/[^0-9]//g')
            if [ -n "$spans_processed_num" ] && [ "$spans_processed_num" != "0" ]; then
                local enforcement_rate=$(echo "$enforced_total $spans_processed_num" | awk '{printf "%.2f", ($1 * 100) / $2}')
                echo -e "Enforcement Rate: ${enforcement_rate}% of spans matched rules"
            fi
        fi
    else
        echo "Cardinality metrics not available (benchmark mode may be disabled)"
    fi
    
    # Processing performance
    echo -e "\n${BLUE}=== Processing Performance ===${NC}"
    local processing_sum=$(echo "$metrics" | grep "otelcol_processor_semconv_processing_duration_milliseconds_sum" | awk '{print $2}')
    local processing_count=$(echo "$metrics" | grep "otelcol_processor_semconv_processing_duration_milliseconds_count" | awk '{print $2}')
    
    if [ -n "$processing_sum" ] && [ -n "$processing_count" ] && [ "$processing_count" != "0" ]; then
        local avg_duration=$(echo "scale=3; $processing_sum / $processing_count" | bc)
        echo -e "Average Processing Duration: ${avg_duration}ms"
        echo -e "Total Processing Calls: $(format_number $processing_count)"
    fi
    
    # Errors
    local errors=$(get_metric_value "otelcol_processor_semconv_errors__errors__total" "" "$metrics")
    if [ -n "$errors" ] && [ "$errors" != "0" ]; then
        echo -e "\n${RED}=== Errors ===${NC}"
        echo -e "Processing Errors: $errors"
    fi
    
    # Pipeline metrics
    echo -e "\n${GREEN}=== Pipeline Metrics ===${NC}"
    local incoming_traces=$(echo "$metrics" | grep 'otelcol_processor_incoming_items__items__total' | grep 'processor="semconv"' | grep 'signal="traces"' | awk '{print $2}' | head -1)
    local outgoing_traces=$(echo "$metrics" | grep 'otelcol_processor_outgoing_items__items__total' | grep 'processor="semconv"' | grep 'signal="traces"' | awk '{print $2}' | head -1)
    
    echo -e "Incoming Traces: $(format_number ${incoming_traces:-0})"
    echo -e "Outgoing Traces: $(format_number ${outgoing_traces:-0})"
    
    if [ -n "$incoming_traces" ] && [ -n "$outgoing_traces" ] && [ "$incoming_traces" != "0" ]; then
        local dropped=$((incoming_traces - outgoing_traces))
        if [ "$dropped" -ne 0 ]; then
            echo -e "Dropped Traces: $(format_number $dropped)"
        fi
    fi
}

# Main monitoring loop
main() {
    echo -e "${BLUE}Starting metrics monitor...${NC}"
    
    if [ "$WATCH_MODE" == "true" ]; then
        echo "Running in watch mode (refresh every ${INTERVAL}s). Press Ctrl+C to exit."
        while true; do
            metrics=$(fetch_metrics)
            display_metrics "$metrics"
            sleep "$INTERVAL"
        done
    else
        metrics=$(fetch_metrics)
        display_metrics "$metrics"
    fi
}

# Handle command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -w|--watch)
            WATCH_MODE="true"
            shift
            ;;
        -i|--interval)
            INTERVAL="$2"
            shift 2
            ;;
        -e|--endpoint)
            METRICS_ENDPOINT="$2"
            shift 2
            ;;
        -h|--help)
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  -w, --watch          Run in watch mode (continuous monitoring)"
            echo "  -i, --interval SEC   Set refresh interval in seconds (default: 5)"
            echo "  -e, --endpoint ADDR  Set metrics endpoint (default: localhost:8888/metrics)"
            echo "  -h, --help          Show this help message"
            echo ""
            echo "Examples:"
            echo "  $0                          # Single snapshot"
            echo "  $0 --watch                  # Continuous monitoring"
            echo "  $0 -w -i 10                 # Watch mode with 10s interval"
            echo "  $0 -e myhost:8888/metrics   # Custom endpoint"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            echo "Use -h or --help for usage information"
            exit 1
            ;;
    esac
done

# Run the main function
main