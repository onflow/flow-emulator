package utils

import (
	"fmt"
	"github.com/logrusorgru/aurora"
	"github.com/onflow/flow-emulator/types"
	sdk "github.com/onflow/flow-go-sdk"
	"github.com/rs/zerolog"
)

func PrintScriptResult(logger *zerolog.Logger, result *types.ScriptResult) {
	if result.Succeeded() {
		logger.Info().
			Str("scriptID", result.ScriptID.String()).
			Uint64("computationUsed", result.ComputationUsed).
			Msg("⭐  Script executed")
	} else {
		logger.Warn().
			Str("scriptID", result.ScriptID.String()).
			Uint64("computationUsed", result.ComputationUsed).
			Msg("❗  Script reverted")
	}

	for _, log := range result.Logs {
		logger.Debug().Msgf(
			"%s %s",
			logPrefix("LOG", result.ScriptID, aurora.BlueFg),
			log,
		)
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
		logger.Info().
			Str("txID", result.TransactionID.String()).
			Uint64("computationUsed", result.ComputationUsed).
			Msg("⭐  Transaction executed")
	} else {
		logger.Warn().
			Str("txID", result.TransactionID.String()).
			Uint64("computationUsed", result.ComputationUsed).
			Msg("❗  Transaction reverted")
	}

	for _, log := range result.Logs {
		logger.Info().Msgf(
			"%s %s",
			logPrefix("LOG", result.TransactionID, aurora.BlueFg),
			log,
		)
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

func logPrefix(prefix string, id sdk.Identifier, color aurora.Color) string {
	prefix = aurora.Colorize(prefix, color|aurora.BoldFm).String()
	shortID := fmt.Sprintf("[%s]", id.String()[:6])
	shortID = aurora.Colorize(shortID, aurora.FaintFm).String()
	return fmt.Sprintf("%s %s", prefix, shortID)
}
