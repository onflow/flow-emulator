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
	"fmt"

	"github.com/logrusorgru/aurora"
	flowsdk "github.com/onflow/flow-go-sdk"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-emulator/types"
)

func PrintScriptResult(logger *zerolog.Logger, result *types.ScriptResult) {
	if result.Succeeded() {
		logger.Debug().
			Str("scriptID", result.ScriptID.String()).
			Uint64("computationUsed", result.ComputationUsed).
			Uint64("memoryEstimate", result.MemoryEstimate).
			Msg("⭐  Script executed")
	} else {
		logger.Warn().
			Str("scriptID", result.ScriptID.String()).
			Uint64("computationUsed", result.ComputationUsed).
			Uint64("memoryEstimate", result.MemoryEstimate).
			Msg("❗  Script reverted")
	}

	if !result.Succeeded() {
		logger.Warn().Msgf(
			"%s %s",
			logPrefix("ERR", result.ScriptID, aurora.RedFg),
			result.Error.Error(),
		)
	}
}

func PrintTransactionResult(logger *zerolog.Logger, result *types.TransactionResult) {
	if result.Succeeded() {
		logger.Debug().
			Str("txID", result.TransactionID.String()).
			Uint64("computationUsed", result.ComputationUsed).
			Uint64("memoryEstimate", result.MemoryEstimate).
			Msg("⭐  Transaction executed")
	} else {
		logger.Warn().
			Str("txID", result.TransactionID.String()).
			Uint64("computationUsed", result.ComputationUsed).
			Uint64("memoryEstimate", result.MemoryEstimate).
			Msg("❗  Transaction reverted")
	}

	for _, event := range result.Events {
		logger.Debug().Msgf(
			"%s %s",
			logPrefix("EVT", result.TransactionID, aurora.GreenFg),
			event,
		)
	}

	if !result.Succeeded() {
		logger.Warn().Msgf(
			"%s %s",
			logPrefix("ERR", result.TransactionID, aurora.RedFg),
			result.Error.Error(),
		)

		if result.Debug != nil {
			logger.Debug().Fields(result.Debug.Meta).Msgf("%s %s", "❗  Transaction Signature Error", result.Debug.Message)
		}
	}
}

// PrintScheduledTransactionResult prints a clearly labeled log entry for scheduled transactions
// execute transactions so they can be easily distinguished from user transactions.
func PrintScheduledTransactionResult(logger *zerolog.Logger, result *types.TransactionResult, scheduledTxID string) {
	if result.Succeeded() {
		logger.Debug().
			Str("systemTxId", result.TransactionID.String()).
			Str("scheduledTxId", scheduledTxID).
			Uint64("computationUsed", result.ComputationUsed).
			Uint64("memoryEstimate", result.MemoryEstimate).
			Str("kind", "scheduled").
			Msg("⏱️  Scheduled transaction executed")
	} else {
		logger.Warn().
			Str("systemTxId", result.TransactionID.String()).
			Str("scheduledTxId", scheduledTxID).
			Uint64("computationUsed", result.ComputationUsed).
			Uint64("memoryEstimate", result.MemoryEstimate).
			Str("kind", "scheduled").
			Msg("❗  Scheduled transaction reverted")
	}

	for _, event := range result.Events {
		logger.Debug().Msgf(
			"%s %s",
			logPrefix("EVT-SYS", result.TransactionID, aurora.GreenFg),
			event,
		)
	}

	if !result.Succeeded() {
		logger.Warn().Msgf(
			"%s %s",
			logPrefix("ERR-SYS", result.TransactionID, aurora.RedFg),
			result.Error.Error(),
		)
	}
}

func logPrefix(prefix string, id flowsdk.Identifier, color aurora.Color) string {
	prefix = aurora.Colorize(prefix, color|aurora.BoldFm).String()
	shortID := fmt.Sprintf("[%s]", id.String()[:6])
	shortID = aurora.Colorize(shortID, aurora.FaintFm).String()
	return fmt.Sprintf("%s %s", prefix, shortID)
}
