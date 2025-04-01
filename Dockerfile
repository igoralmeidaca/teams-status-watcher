# Use a small base image
FROM golang:1.23 as builder

# Set the working directory inside the container
WORKDIR /app

# Copy Go modules and dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the application
COPY . .

# Build the Go application
RUN go build -o teams-status-watcher

# Use a minimal runtime image
FROM debian:bookworm-slim

# Set environment variables (optional)
ENV LOGS_PATH="/logs"

# Copy the compiled binary from the builder stage
COPY --from=builder /app/teams-status-watcher /usr/local/bin/teams-status-watcher

# Set execution command
CMD ["/usr/local/bin/teams-status-watcher"]
