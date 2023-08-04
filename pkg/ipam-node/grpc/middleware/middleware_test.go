/*
 Copyright 2023, NVIDIA CORPORATION & AFFILIATES
 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at
     http://www.apache.org/licenses/LICENSE-2.0
 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package middleware_test

import (
	"context"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc"

	"github.com/Mellanox/nvidia-k8s-ipam/pkg/ipam-node/grpc/middleware"
)

type fakeHandler struct {
	IsCalled      bool
	CalledWithCtx context.Context
	CalledWithReq interface{}
	Error         error
}

func (fh *fakeHandler) Handle(ctx context.Context, req interface{}) (interface{}, error) {
	fh.IsCalled = true
	fh.CalledWithCtx = ctx
	fh.CalledWithReq = req
	return nil, fh.Error
}

const testFullMethod = "foobar"

var testUnaryServerInfo = &grpc.UnaryServerInfo{
	FullMethod: testFullMethod,
}

var _ = Describe("Middleware tests", func() {
	var (
		ctx context.Context
	)
	BeforeEach(func() {
		ctx = context.Background()
	})
	Describe("Logger middleware", func() {
		It("Set", func() {
			handler := fakeHandler{}
			_, err := middleware.SetLoggerMiddleware(ctx, nil, testUnaryServerInfo, handler.Handle)
			Expect(err).NotTo(HaveOccurred())
			Expect(handler.IsCalled).To(BeTrue())
			Expect(logr.FromContextOrDiscard(handler.CalledWithCtx)).NotTo(BeNil())
		})
		It("Req/Resp", func() {
			handler := fakeHandler{}
			ctx = logr.NewContext(ctx, logr.Discard())
			_, err := middleware.LogCallMiddleware(ctx, nil, testUnaryServerInfo, handler.Handle)
			Expect(err).NotTo(HaveOccurred())
			Expect(handler.IsCalled).To(BeTrue())
		})
	})
})
