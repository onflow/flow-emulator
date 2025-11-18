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
// This config is applied to all remote gRPC connections to handle network flakiness and
// rate limiting from remote access nodes.
//
// Note: We explicitly list service names instead of using the [{}] wildcard matcher because
// empirical testing showed inconsistent matching behavior across environments. Explicit service
// names ensure the retry policy is reliably applied in all environments including CI.
const DefaultGRPCServiceConfig = `{
	"methodConfig": [{
		"name": [
			{"service": "flow.access.AccessAPI"},
			{"service": "flow.executiondata.ExecutionDataAPI"}
		],
		"retryPolicy": {
			"maxAttempts": 10,
			"initialBackoff": "1s",
			"maxBackoff": "30s",
			"backoffMultiplier": 2,
			"retryableStatusCodes": ["UNAVAILABLE", "RESOURCE_EXHAUSTED", "UNKNOWN"]
		}
	}]
}`
