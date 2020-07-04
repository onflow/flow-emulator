package unittest

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-emulator/types"
)

func AssertFVMErrorType(t *testing.T, expected interface{}, err error) bool {
	require.IsType(t, err, &types.FlowError{})

	flowError := err.(*types.FlowError)

	if !assert.IsType(t, expected, flowError.FlowError) {
		t.Log(flowError.FlowError.Error())
		return false
	}

	return true
}
