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

// DefaultGRPCServiceConfig configures native gRPC retries for transient failures.
// The empty object wildcard [{}] matches all services and methods.
const DefaultGRPCServiceConfig = `{
	"methodConfig": [{
		"name": [],
		"retryPolicy": {
			"maxAttempts": 8,
			"initialBackoff": "1s",
			"maxBackoff": "30s",
			"backoffMultiplier": 2,
			"retryableStatusCodes": ["UNAVAILABLE", "RESOURCE_EXHAUSTED", "UNKNOWN"]
		}
	}]
}`

