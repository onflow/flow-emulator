/*
 * Flow Emulator
 *
 * Copyright Flow Foundation
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package utils

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	defaultMaxAttempts    = 10
	defaultInitialBackoff = 1 * time.Second
	defaultMaxBackoff     = 30 * time.Second
	defaultBackoffFactor  = 2.0
)

// DefaultGRPCRetryInterceptor returns a unary client interceptor that retries
// transient failures with exponential backoff. Unlike native gRPC retries, this
// ignores server pushback headers (grpc-retry-pushback-ms) so rate-limited calls
// will retry client-side rather than fail immediately.
func DefaultGRPCRetryInterceptor() grpc.DialOption {
	return grpc.WithChainUnaryInterceptor(retryInterceptor)
}

func retryInterceptor(
	ctx context.Context,
	method string,
	req, reply any,
	cc *grpc.ClientConn,
	invoker grpc.UnaryInvoker,
	opts ...grpc.CallOption,
) error {
	var lastErr error
	backoff := defaultInitialBackoff

	for attempt := 0; attempt < defaultMaxAttempts; attempt++ {
		if attempt > 0 {
			// Wait before retry
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return ctx.Err()
			}
			// Exponential backoff with cap
			backoff = time.Duration(float64(backoff) * defaultBackoffFactor)
			if backoff > defaultMaxBackoff {
				backoff = defaultMaxBackoff
			}
		}

		lastErr = invoker(ctx, method, req, reply, cc, opts...)
		if lastErr == nil {
			return nil
		}

		// Check if error is retryable
		code := status.Code(lastErr)
		if !isRetryableCode(code) {
			return lastErr
		}
	}

	return lastErr
}

func isRetryableCode(code codes.Code) bool {
	switch code {
	case codes.Unavailable, codes.ResourceExhausted, codes.Unknown:
		return true
	default:
		return false
	}
}
