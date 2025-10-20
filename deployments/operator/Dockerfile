# Multi-stage build for Kubernetes operator
FROM registry.access.redhat.com/ubi9/ubi:latest AS builder

# Install build dependencies and Go 1.24
RUN dnf install -y git ca-certificates wget tar && dnf clean all

# Install Go 1.24 manually since UBI9 go-toolset only has 1.21
RUN ARCH=$(uname -m) && \
    if [ "$ARCH" = "x86_64" ]; then GOARCH="amd64"; elif [ "$ARCH" = "aarch64" ]; then GOARCH="arm64"; else GOARCH="amd64"; fi && \
    wget -O go1.24.5.linux-${GOARCH}.tar.gz https://go.dev/dl/go1.24.5.linux-${GOARCH}.tar.gz && \
    tar -C /usr/local -xzf go1.24.5.linux-${GOARCH}.tar.gz && \
    rm go1.24.5.linux-${GOARCH}.tar.gz

# Set Go environment
ENV PATH="/usr/local/go/bin:${PATH}"
ENV GOROOT="/usr/local/go"

# Set working directory
WORKDIR /src

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the operator binary
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags "-X main.version=v0.4.1 -X main.commit=$(git rev-parse --short HEAD) -X main.date=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    -o /bin/operator ./cmd/operator

# Final stage
FROM registry.access.redhat.com/ubi9/ubi-minimal:latest

# Install runtime dependencies
RUN microdnf install -y ca-certificates && microdnf clean all

# Create non-root user
RUN groupadd -g 1000 jira && \
    useradd -r -u 1000 -g jira -s /bin/sh jira

# Create working directories
RUN mkdir -p /workspace /tmp && \
    chown -R jira:jira /workspace /tmp

# Copy binary from builder
COPY --from=builder /bin/operator /bin/operator
RUN chmod +x /bin/operator

# Switch to non-root user
USER jira

# Set working directory
WORKDIR /workspace

# Expose metrics and health check ports
EXPOSE 8080 8081

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD /bin/operator --help || exit 1

# Default command
CMD ["/bin/operator"]