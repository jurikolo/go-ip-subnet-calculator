# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app
RUN apk add --no-cache git
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main .

# Run stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates
RUN addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup
WORKDIR /app
COPY --from=builder /app/main .
COPY index.html .
RUN chown appuser:appgroup main index.html && \
    chmod +x main
USER appuser
ENV GO_SUBNET_CALCULATOR_PORT=8080
EXPOSE $GO_SUBNET_CALCULATOR_PORT

CMD ["./main"]