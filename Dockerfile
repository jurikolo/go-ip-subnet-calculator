# Build stage
FROM golang:1.24-alpine AS builder

# Set working directory
WORKDIR /app

# Install git (required for some Go modules)
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main .

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

# Create non-root user
RUN addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup

WORKDIR /root/

# Copy the binary from builder stage
COPY --from=builder /app/main .

# Change ownership and set execute permissions
RUN chown appuser:appgroup main && \
    chmod +x main

# Switch to non-root user
USER appuser

# Set default port environment variable
ENV GO_SUBNET_CALCULATOR_PORT=8080

# Expose the port
EXPOSE $GO_SUBNET_CALCULATOR_PORT

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:${GO_SUBNET_CALCULATOR_PORT}/health || exit 1

# Command to run the application
CMD ["./main"]