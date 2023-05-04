# Build the image
FROM golang:1.20 as builder

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY . /workspace

# Build with make to apply all build logic defined in Makefile
RUN make build
# Build host-local cni
RUN git clone https://github.com/containernetworking/plugins.git ; cd plugins ; git checkout v1.2.0 -b v1.2.0
RUN cd plugins ; go build -o plugins/bin/host-local ./plugins/ipam/host-local

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/base-debian11:latest
WORKDIR /
COPY --from=builder /workspace/build/ipam-controller .
COPY --from=builder /workspace/build/ipam-node .
COPY --from=builder /workspace/build/nv-ipam .
COPY --from=builder /workspace/plugins/plugins/bin/host-local .
