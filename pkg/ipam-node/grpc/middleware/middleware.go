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

package middleware

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"k8s.io/klog/v2"
)

// SetLoggerMiddleware creates logger instance with additional information and saves it to req context
func SetLoggerMiddleware(ctx context.Context, req interface{},
	info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	ctx = logr.NewContext(ctx,
		klog.NewKlogr().WithValues("method", info.FullMethod, "reqID", uuid.New().String()))
	return handler(ctx, req)
}

// LogCallMiddleware log request and response with configured logger
func LogCallMiddleware(ctx context.Context, req interface{},
	_ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	reqLogger := logr.FromContextOrDiscard(ctx)
	startTime := time.Now()
	reqLogger.V(1).Info("REQUEST")
	resp, err := handler(ctx, req)
	reqLogger = reqLogger.WithValues(
		"call_duration_sec", time.Since(startTime).Seconds())
	if err != nil {
		reqLogger.Error(err, "ERROR RESPONSE")
	} else {
		reqLogger.V(1).Info("RESPONSE")
	}
	return resp, err
}
