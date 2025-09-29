# Debug version of Dockerfile
FROM golang:1.24-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git

COPY go.mod ./
RUN go mod download

COPY . .

# Build with debug info
RUN CGO_ENABLED=0 GOOS=linux go build -v -o main .
RUN ls -la main && file main

# Use alpine instead of scratch for debugging
FROM alpine:latest

# Install debugging tools
RUN apk --no-cache add ca-certificates file strace ldd

# Create app directory
RUN mkdir -p /app

WORKDIR /app

# Copy binary
COPY --from=builder /app/main ./main

# Set permissions explicitly
RUN chmod 755 main

# Show binary details
RUN ls -la main && file main

# Test the binary
RUN echo "Testing binary:" && ./main --version || echo "Binary test completed"

ENV GO_SUBNET_CALCULATOR_PORT=8080

EXPOSE 8080

# Use explicit path
CMD ["/app/main"]