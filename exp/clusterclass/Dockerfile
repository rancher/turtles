# Build the manager binary
ARG builder_image

# Build architecture
ARG ARCH

FROM ${builder_image} as builder
WORKDIR /workspace

ARG goproxy=https://proxy.golang.org
ENV GOPROXY=$goproxy

COPY go.mod go.mod
COPY go.sum go.sum

RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy the go source
COPY ./ ./

ARG package=.
ARG ARCH
ARG ldflags

RUN --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 GOOS=linux GOARCH=${ARCH} \
    go build -trimpath -ldflags "${ldflags} -extldflags '-static'" \
    -o manager ${package}

FROM gcr.io/distroless/static:nonroot-${ARCH}
LABEL org.opencontainers.image.source=https://github.com/rancher/turtles/exp/clusterclass
WORKDIR /
COPY --from=builder /workspace/manager .
# Use uid of nonroot user (65532) because kubernetes expects numeric user when applying pod security policies
USER 65532
ENTRYPOINT ["/manager"]
