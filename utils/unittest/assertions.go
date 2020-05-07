package unittest

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-emulator/types"
)

func AssertFlowVMErrorType(t *testing.T, err error, expected interface{}) bool {

	assert.IsType(t, err, &types.FlowError{})

	flowError := err.(*types.FlowError)

	return assert.IsType(t, flowError.FlowError, expected)
}
