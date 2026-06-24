# Use a multi-stage build to keep the final image small
# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy go.mod and go.sum files
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -o webhook ./cmd/webhook/main.go

# Final stage
FROM alpine:3.18

# Add CA certificates for secure communication
RUN apk add --no-cache ca-certificates

# Create a non-privileged user to run the application
RUN addgroup -S webhook && adduser -S webhook -G webhook
USER webhook

WORKDIR /home/webhook

# Copy the binary from the builder stage
COPY --from=builder /app/webhook .

# Expose the port the application listens on
EXPOSE 8443

# Start the application
CMD ["./webhook"]
