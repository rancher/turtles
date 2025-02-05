# syntax=docker/dockerfile:1.4

# Copyright © 2023 - 2024 SUSE LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Build the manager binary
ARG builder_image

FROM --platform=$BUILDPLATFORM ${builder_image} as builder
WORKDIR /workspace

# Run this with docker build --build-arg goproxy=$(go env GOPROXY) to override the goproxy
ARG goproxy=https://proxy.golang.org
# Run this with docker build --build-arg package=./controlplane or --build-arg package=./bootstrap
ENV GOPROXY=$goproxy

COPY ./ ./

# Build
ARG package=.
ARG ldflags
ARG TARGETOS TARGETARCH

# Do not force rebuild of up-to-date packages (do not use -a) and use the compiler cache folder
RUN --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags "${ldflags} -extldflags '-static'" \
    -o manager ${package}


FROM --platform=$BUILDPLATFORM ${builder_image} as day2-operations-builder
WORKDIR /workspace

# Run this with docker build --build-arg goproxy=$(go env GOPROXY) to override the goproxy
ARG goproxy=https://proxy.golang.org
# Run this with docker build --build-arg package=./exp/day2
ENV GOPROXY=$goproxy

# Copy the sources
COPY ./ ./

# Build
ARG ldflags
ARG TARGETOS TARGETARCH

# Do not force rebuild of up-to-date packages (do not use -a) and use the compiler cache folder
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    sh -c "cd exp/day2 && ls && go build -trimpath -ldflags \"${ldflags} -extldflags '-static'\" -o manager ${package}"

FROM --platform=$BUILDPLATFORM ${builder_image} as clusterclass-operations-builder
WORKDIR /workspace

# Run this with docker build --build-arg goproxy=$(go env GOPROXY) to override the goproxy
ARG goproxy=https://proxy.golang.org
# Run this with docker build --build-arg package=./exp/etcdrestore
ENV GOPROXY=$goproxy

# Copy the sources
COPY ./ ./

# Build
ARG ldflags
ARG TARGETOS TARGETARCH

# Do not force rebuild of up-to-date packages (do not use -a) and use the compiler cache folder
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    sh -c "cd exp/clusterclass && ls && go build -trimpath -ldflags \"${ldflags} -extldflags '-static'\" -o manager ${package}"

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot
LABEL org.opencontainers.image.source=https://github.com/rancher/turtles
WORKDIR /
COPY --from=builder /workspace/manager .
COPY --from=day2-operations-builder /workspace/exp/day2/manager turtles-day2-operations
COPY --from=clusterclass-operations-builder /workspace/exp/clusterclass/manager turtles-clusterclass-operations
# Use uid of nonroot user (65532) because kubernetes expects numeric user when applying pod security policies
USER 65532
ENTRYPOINT ["/manager"]
