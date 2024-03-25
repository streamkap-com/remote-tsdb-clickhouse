# Start from a base Golang image
FROM golang:1.20 AS builder

# Set the working directory inside the container
WORKDIR /app

# Copy the Go modules files
COPY go.mod go.sum ./

# Download the Go module dependencies
RUN go mod download

# Copy the rest of the project files
COPY . .

# Build the Go application
RUN CGO_ENABLED=0 GOOS=linux go build -o remote-tsdb-clickhouse

# Start a new stage for the final image
FROM alpine:latest

# Install any necessary dependencies in the final image
RUN apk --no-cache add ca-certificates

# Copy the built Go binary from the builder stage
COPY --from=builder /app/remote-tsdb-clickhouse /usr/local/bin/remote-tsdb-clickhouse

# Expose the desired port
EXPOSE 9131

# Set the entry point for the container
ENTRYPOINT ["remote-tsdb-clickhouse"]