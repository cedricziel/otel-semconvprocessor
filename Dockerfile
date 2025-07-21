# Build stage
FROM golang:1.23-alpine AS builder

# Install required packages
RUN apk add --no-cache git make

# Set working directory
WORKDIR /build

# Install OpenTelemetry Collector Builder
RUN go install go.opentelemetry.io/collector/cmd/builder@v0.130.0

# Copy builder config first
COPY builder-config.yaml ./

# Copy processor module
COPY processors/semconvprocessor/ ./processors/semconvprocessor/

# Copy the entire project
COPY . .

# Build the custom collector
RUN builder --config=builder-config.yaml

# Runtime stage
FROM alpine:3.19

# Install ca-certificates for HTTPS support
RUN apk --no-cache add ca-certificates

# Create non-root user
RUN addgroup -g 10001 -S otel && \
    adduser -u 10001 -S otel -G otel

# Copy the built collector binary
COPY --from=builder /build/otelcol-semconv/otelcol-semconv /otelcol-semconv

# Set ownership
RUN chown -R otel:otel /otelcol-semconv

# Use non-root user
USER otel

# Expose typical OpenTelemetry ports
# 4317 - OTLP gRPC receiver
# 4318 - OTLP HTTP receiver
# 8888 - Prometheus metrics
# 13133 - Health check
# 55679 - zPages
EXPOSE 4317 4318 8888 13133 55679

# Set the entrypoint
ENTRYPOINT ["/otelcol-semconv"]

# Default command arguments (can be overridden)
CMD ["--config=/etc/otelcol/config.yaml"]