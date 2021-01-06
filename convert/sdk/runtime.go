/*
 * Flow Emulator
 *
 * Copyright 2019-2020 Dapper Labs, Inc.
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

package sdk

import (
	"fmt"
	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	sdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go/model/flow"
)

func RuntimeEventToSDK(runtimeEvent flow.Event, txID sdk.Identifier, txIndex int, eventIndex int) (sdk.Event, error) {
	v, err := jsoncdc.Decode(runtimeEvent.Payload)

	if err != nil {
		return sdk.Event{}, fmt.Errorf("could not decode cadence event")
	}

	return sdk.Event{
		Type:             string(runtimeEvent.Type),
		TransactionID:    txID,
		TransactionIndex: txIndex,
		EventIndex:       eventIndex,
		Value:            v.(cadence.Event),
	}, nil
}

func RuntimeEventsToSDK(runtimeEvents []flow.Event, txID sdk.Identifier, txIndex int) ([]sdk.Event, error) {
	ret := make([]sdk.Event, len(runtimeEvents))
	for i, runtimeEvent := range runtimeEvents {
		e, err := RuntimeEventToSDK(runtimeEvent, txID, txIndex, i)
		if err != nil {
			return nil, err
		}
		ret[i] = e
	}
	return ret, nil
}
