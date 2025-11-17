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

// DefaultGRPCServiceConfig provides automatic retry configuration for transient gRPC errors.
// This config is applied to all remote gRPC connections to handle network flakiness in CI
// and other environments.
//
// Retries on:
// - UNAVAILABLE: Service temporarily unavailable (e.g., node restarting)
// - RESOURCE_EXHAUSTED: Rate limiting from remote node
// - UNKNOWN: Connection failures, DNS issues, and other network errors
//
// Note: We only retry on clearly transient network/availability errors.
// We do NOT retry on INTERNAL (programming errors), ABORTED (conflicts),
// or DEADLINE_EXCEEDED (to avoid cascading failures on slow services).
const DefaultGRPCServiceConfig = `{
	"methodConfig": [{
		"name": [{"service": ""}],
		"retryPolicy": {
			"maxAttempts": 5,
			"initialBackoff": "0.1s",
			"maxBackoff": "30s",
			"backoffMultiplier": 2,
			"retryableStatusCodes": ["UNAVAILABLE", "RESOURCE_EXHAUSTED", "UNKNOWN"]
		}
	}]
}`
