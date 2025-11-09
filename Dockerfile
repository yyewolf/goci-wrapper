# Multi-stage build for maximum security
# Stage 1: Build the application
FROM golang:1.25-alpine AS builder

# Install security updates and build dependencies
RUN apk update && \
    apk add --no-cache ca-certificates git tzdata && \
    apk upgrade && \
    rm -rf /var/cache/apk/*

# Create a non-root user for building
RUN addgroup -g 10001 -S nonroot && \
    adduser -u 10001 -S -G nonroot -h /home/nonroot nonroot

# Set working directory
WORKDIR /build

# Copy go mod files first for better caching
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download && go mod verify

# Copy source code
COPY . .

# Build the binary with security flags
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -a -installsuffix cgo \
    -ldflags='-w -s -extldflags "-static"' \
    -o goci-wrapper .

# Verify the binary
RUN chmod +x goci-wrapper

# Create custom passwd and group files for the scratch image
RUN echo "goci:x:65534:65534:Goci Wrapper User:/:/sbin/nologin" > /tmp/passwd && \
    echo "goci:x:65534:" > /tmp/group

# Stage 2: Create minimal runtime image
FROM scratch

# Copy CA certificates for HTTPS requests to registries
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy timezone data (needed for Go time operations)
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Copy custom passwd and group files
COPY --from=builder /tmp/passwd /etc/passwd
COPY --from=builder /tmp/group /etc/group

# Copy the binary
COPY --from=builder --chown=65534:65534 /build/goci-wrapper /usr/local/bin/goci-wrapper

# Use non-root user (UID 65534 is commonly used for 'nobody')
USER 65534:65534

# Set security and metadata labels
LABEL \
    org.opencontainers.image.title="Goci Wrapper" \
    org.opencontainers.image.description="Dynamic OCI image wrapper service" \
    org.opencontainers.image.version="1.0" \
    org.opencontainers.image.vendor="Yewolf" \
    org.opencontainers.image.licenses="MIT" \
    org.opencontainers.image.source="https://github.com/yyewolf/goci-wrapper" \
    security.non-root="true" \
    security.no-shell="true" \
    security.scratch-based="true" \
    security.user="goci" \
    security.uid="65534"

# Expose port (documentation only, doesn't actually publish)
EXPOSE 5000

# Run as non-root
ENTRYPOINT ["/usr/local/bin/goci-wrapper"]