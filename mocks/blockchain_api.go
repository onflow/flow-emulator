// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/dapperlabs/flow-emulator (interfaces: BlockchainAPI)

// Package mocks is a generated GoMock package.
package mocks

import (
	flow_emulator "github.com/dapperlabs/flow-emulator"
	types "github.com/dapperlabs/flow-emulator/types"
	flow_go_sdk "github.com/dapperlabs/flow-go-sdk"
	gomock "github.com/golang/mock/gomock"
	reflect "reflect"
)

// MockBlockchainAPI is a mock of BlockchainAPI interface
type MockBlockchainAPI struct {
	ctrl     *gomock.Controller
	recorder *MockBlockchainAPIMockRecorder
}

// MockBlockchainAPIMockRecorder is the mock recorder for MockBlockchainAPI
type MockBlockchainAPIMockRecorder struct {
	mock *MockBlockchainAPI
}

// NewMockBlockchainAPI creates a new mock instance
func NewMockBlockchainAPI(ctrl *gomock.Controller) *MockBlockchainAPI {
	mock := &MockBlockchainAPI{ctrl: ctrl}
	mock.recorder = &MockBlockchainAPIMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockBlockchainAPI) EXPECT() *MockBlockchainAPIMockRecorder {
	return m.recorder
}

// AddTransaction mocks base method
func (m *MockBlockchainAPI) AddTransaction(arg0 flow_go_sdk.Transaction) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AddTransaction", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// AddTransaction indicates an expected call of AddTransaction
func (mr *MockBlockchainAPIMockRecorder) AddTransaction(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddTransaction", reflect.TypeOf((*MockBlockchainAPI)(nil).AddTransaction), arg0)
}

// CommitBlock mocks base method
func (m *MockBlockchainAPI) CommitBlock() (*types.Block, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CommitBlock")
	ret0, _ := ret[0].(*types.Block)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CommitBlock indicates an expected call of CommitBlock
func (mr *MockBlockchainAPIMockRecorder) CommitBlock() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CommitBlock", reflect.TypeOf((*MockBlockchainAPI)(nil).CommitBlock))
}

// ExecuteAndCommitBlock mocks base method
func (m *MockBlockchainAPI) ExecuteAndCommitBlock() (*types.Block, []flow_emulator.TransactionResult, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ExecuteAndCommitBlock")
	ret0, _ := ret[0].(*types.Block)
	ret1, _ := ret[1].([]flow_emulator.TransactionResult)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// ExecuteAndCommitBlock indicates an expected call of ExecuteAndCommitBlock
func (mr *MockBlockchainAPIMockRecorder) ExecuteAndCommitBlock() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ExecuteAndCommitBlock", reflect.TypeOf((*MockBlockchainAPI)(nil).ExecuteAndCommitBlock))
}

// ExecuteBlock mocks base method
func (m *MockBlockchainAPI) ExecuteBlock() ([]flow_emulator.TransactionResult, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ExecuteBlock")
	ret0, _ := ret[0].([]flow_emulator.TransactionResult)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ExecuteBlock indicates an expected call of ExecuteBlock
func (mr *MockBlockchainAPIMockRecorder) ExecuteBlock() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ExecuteBlock", reflect.TypeOf((*MockBlockchainAPI)(nil).ExecuteBlock))
}

// ExecuteNextTransaction mocks base method
func (m *MockBlockchainAPI) ExecuteNextTransaction() (flow_emulator.TransactionResult, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ExecuteNextTransaction")
	ret0, _ := ret[0].(flow_emulator.TransactionResult)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ExecuteNextTransaction indicates an expected call of ExecuteNextTransaction
func (mr *MockBlockchainAPIMockRecorder) ExecuteNextTransaction() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ExecuteNextTransaction", reflect.TypeOf((*MockBlockchainAPI)(nil).ExecuteNextTransaction))
}

// ExecuteScript mocks base method
func (m *MockBlockchainAPI) ExecuteScript(arg0 []byte) (flow_emulator.ScriptResult, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ExecuteScript", arg0)
	ret0, _ := ret[0].(flow_emulator.ScriptResult)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ExecuteScript indicates an expected call of ExecuteScript
func (mr *MockBlockchainAPIMockRecorder) ExecuteScript(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ExecuteScript", reflect.TypeOf((*MockBlockchainAPI)(nil).ExecuteScript), arg0)
}

// ExecuteScriptAtBlock mocks base method
func (m *MockBlockchainAPI) ExecuteScriptAtBlock(arg0 []byte, arg1 uint64) (flow_emulator.ScriptResult, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ExecuteScriptAtBlock", arg0, arg1)
	ret0, _ := ret[0].(flow_emulator.ScriptResult)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ExecuteScriptAtBlock indicates an expected call of ExecuteScriptAtBlock
func (mr *MockBlockchainAPIMockRecorder) ExecuteScriptAtBlock(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ExecuteScriptAtBlock", reflect.TypeOf((*MockBlockchainAPI)(nil).ExecuteScriptAtBlock), arg0, arg1)
}

// GetAccount mocks base method
func (m *MockBlockchainAPI) GetAccount(arg0 flow_go_sdk.Address) (*flow_go_sdk.Account, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetAccount", arg0)
	ret0, _ := ret[0].(*flow_go_sdk.Account)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetAccount indicates an expected call of GetAccount
func (mr *MockBlockchainAPIMockRecorder) GetAccount(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetAccount", reflect.TypeOf((*MockBlockchainAPI)(nil).GetAccount), arg0)
}

// GetAccountAtBlock mocks base method
func (m *MockBlockchainAPI) GetAccountAtBlock(arg0 flow_go_sdk.Address, arg1 uint64) (*flow_go_sdk.Account, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetAccountAtBlock", arg0, arg1)
	ret0, _ := ret[0].(*flow_go_sdk.Account)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetAccountAtBlock indicates an expected call of GetAccountAtBlock
func (mr *MockBlockchainAPIMockRecorder) GetAccountAtBlock(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetAccountAtBlock", reflect.TypeOf((*MockBlockchainAPI)(nil).GetAccountAtBlock), arg0, arg1)
}

// GetBlockByHeight mocks base method
func (m *MockBlockchainAPI) GetBlockByHeight(arg0 uint64) (*types.Block, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetBlockByHeight", arg0)
	ret0, _ := ret[0].(*types.Block)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetBlockByHeight indicates an expected call of GetBlockByHeight
func (mr *MockBlockchainAPIMockRecorder) GetBlockByHeight(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetBlockByHeight", reflect.TypeOf((*MockBlockchainAPI)(nil).GetBlockByHeight), arg0)
}

// GetBlockByID mocks base method
func (m *MockBlockchainAPI) GetBlockByID(arg0 flow_go_sdk.Identifier) (*types.Block, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetBlockByID", arg0)
	ret0, _ := ret[0].(*types.Block)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetBlockByID indicates an expected call of GetBlockByID
func (mr *MockBlockchainAPIMockRecorder) GetBlockByID(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetBlockByID", reflect.TypeOf((*MockBlockchainAPI)(nil).GetBlockByID), arg0)
}

// GetEventsByHeight mocks base method
func (m *MockBlockchainAPI) GetEventsByHeight(arg0 uint64, arg1 string) ([]flow_go_sdk.Event, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetEventsByHeight", arg0, arg1)
	ret0, _ := ret[0].([]flow_go_sdk.Event)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetEventsByHeight indicates an expected call of GetEventsByHeight
func (mr *MockBlockchainAPIMockRecorder) GetEventsByHeight(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetEventsByHeight", reflect.TypeOf((*MockBlockchainAPI)(nil).GetEventsByHeight), arg0, arg1)
}

// GetLatestBlock mocks base method
func (m *MockBlockchainAPI) GetLatestBlock() (*types.Block, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetLatestBlock")
	ret0, _ := ret[0].(*types.Block)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetLatestBlock indicates an expected call of GetLatestBlock
func (mr *MockBlockchainAPIMockRecorder) GetLatestBlock() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetLatestBlock", reflect.TypeOf((*MockBlockchainAPI)(nil).GetLatestBlock))
}

// GetTransaction mocks base method
func (m *MockBlockchainAPI) GetTransaction(arg0 flow_go_sdk.Identifier) (*flow_go_sdk.Transaction, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetTransaction", arg0)
	ret0, _ := ret[0].(*flow_go_sdk.Transaction)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetTransaction indicates an expected call of GetTransaction
func (mr *MockBlockchainAPIMockRecorder) GetTransaction(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetTransaction", reflect.TypeOf((*MockBlockchainAPI)(nil).GetTransaction), arg0)
}

// GetTransactionResult mocks base method
func (m *MockBlockchainAPI) GetTransactionResult(arg0 flow_go_sdk.Identifier) (*flow_go_sdk.TransactionResult, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetTransactionResult", arg0)
	ret0, _ := ret[0].(*flow_go_sdk.TransactionResult)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetTransactionResult indicates an expected call of GetTransactionResult
func (mr *MockBlockchainAPIMockRecorder) GetTransactionResult(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetTransactionResult", reflect.TypeOf((*MockBlockchainAPI)(nil).GetTransactionResult), arg0)
}

// RootAccountAddress mocks base method
func (m *MockBlockchainAPI) RootAccountAddress() flow_go_sdk.Address {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RootAccountAddress")
	ret0, _ := ret[0].(flow_go_sdk.Address)
	return ret0
}

// RootAccountAddress indicates an expected call of RootAccountAddress
func (mr *MockBlockchainAPIMockRecorder) RootAccountAddress() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RootAccountAddress", reflect.TypeOf((*MockBlockchainAPI)(nil).RootAccountAddress))
}

// RootKey mocks base method
func (m *MockBlockchainAPI) RootKey() flow_emulator.RootKey {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RootKey")
	ret0, _ := ret[0].(flow_emulator.RootKey)
	return ret0
}

// RootKey indicates an expected call of RootKey
func (mr *MockBlockchainAPIMockRecorder) RootKey() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RootKey", reflect.TypeOf((*MockBlockchainAPI)(nil).RootKey))
}
