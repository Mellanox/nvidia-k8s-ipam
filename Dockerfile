# Copyright 2025 NVIDIA CORPORATION & AFFILIATES
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
#
# SPDX-License-Identifier: Apache-2.0

ARG BASE_IMAGE_GO_DISTROLESS

# Build the image
FROM golang:1.26.0 AS builder

ARG GOPROXY
ENV GOPROXY=$GOPROXY

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

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM ${BASE_IMAGE_GO_DISTROLESS:-nvcr.io/nvidia/distroless/go:v3.2.1}
COPY . /src
WORKDIR /
COPY --from=builder /workspace/build/ipam-controller .
COPY --from=builder /workspace/build/ipam-node .
COPY --from=builder /workspace/build/nv-ipam .
