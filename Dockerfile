# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Add required dependencies for building
RUN apk add --no-cache git

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the application
# CGO_ENABLED=0 ensures a statically linked binary
RUN CGO_ENABLED=0 GOOS=linux go build -o /api ./cmd/api/main.go

# Final stage
FROM alpine:latest

# Add ca-certificates for HTTPS requests and tzdata for timezones
RUN apk --no-cache add ca-certificates tzdata

WORKDIR /root/

# Copy the pre-built binary file from the previous stage
COPY --from=builder /api .

# Expose port 8080 to the outside world
EXPOSE 8080

# Command to run the executable
CMD ["./api"]
