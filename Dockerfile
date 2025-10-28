# Build stage
FROM golang:1.23-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make ca-certificates

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o terranovate .

# Runtime stage
FROM hashicorp/terraform:1.9

# Install git (required for git-based modules)
RUN apk add --no-cache git ca-certificates

# Copy binary from builder
COPY --from=builder /build/terranovate /usr/local/bin/terranovate

# Create working directory
WORKDIR /workspace

# Set terraform binary path
ENV TERRAFORM_BIN=/usr/local/bin/terraform

# Run as non-root user
RUN addgroup -S terranovate && adduser -S terranovate -G terranovate
USER terranovate

ENTRYPOINT ["terranovate"]
CMD ["--help"]
