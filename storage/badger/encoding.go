package badger

import (
	"bytes"
	"encoding/gob"

	"github.com/dapperlabs/cadence"
	"github.com/dapperlabs/flow-go-sdk"
	vm "github.com/dapperlabs/flow-go/engine/execution/computation/virtualmachine"
	model "github.com/dapperlabs/flow-go/model/flow"

	"github.com/dapperlabs/flow-emulator/types"
)

func init() {
	// TODO: Revisit if gob is the best encoding option for the emulator

	// Register Type Information
	gob.Register(cadence.AnyType{})
	gob.Register(cadence.AnyStructType{})
	gob.Register(cadence.AnyResourceType{})
	gob.Register(cadence.OptionalType{})
	gob.Register(cadence.Variable{})
	gob.Register(cadence.VoidType{})
	gob.Register(cadence.BoolType{})
	gob.Register(cadence.StringType{})
	gob.Register(cadence.BytesType{})
	gob.Register(cadence.AddressType{})
	gob.Register(cadence.IntType{})
	gob.Register(cadence.Int8Type{})
	gob.Register(cadence.Int16Type{})
	gob.Register(cadence.Int32Type{})
	gob.Register(cadence.Int64Type{})
	gob.Register(cadence.Int128Type{})
	gob.Register(cadence.Int256Type{})
	gob.Register(cadence.UIntType{})
	gob.Register(cadence.UInt8Type{})
	gob.Register(cadence.UInt16Type{})
	gob.Register(cadence.UInt32Type{})
	gob.Register(cadence.UInt64Type{})
	gob.Register(cadence.UInt128Type{})
	gob.Register(cadence.UInt256Type{})
	gob.Register(cadence.Word8Type{})
	gob.Register(cadence.Word16Type{})
	gob.Register(cadence.Word32Type{})
	gob.Register(cadence.Word64Type{})
	gob.Register(cadence.Fix64Type{})
	gob.Register(cadence.UFix64Type{})
	gob.Register(cadence.VariableSizedArrayType{})
	gob.Register(cadence.ConstantSizedArrayType{})
	gob.Register(cadence.DictionaryType{})
	gob.Register(cadence.Field{})
	gob.Register(cadence.Parameter{})
	gob.Register(cadence.StructType{})
	gob.Register(cadence.ResourceType{})
	gob.Register(cadence.EventType{})
	gob.Register(cadence.Function{})
	gob.Register(cadence.ResourcePointer{})
	gob.Register(cadence.StructPointer{})
	gob.Register(cadence.EventPointer{})

	// Register Values
	gob.Register(cadence.Void{})
	gob.Register(cadence.Optional{})
	gob.Register(cadence.Bool(false))
	gob.Register(cadence.String(""))
	gob.Register(cadence.Bytes{})
	gob.Register(cadence.Address{})
	gob.Register(cadence.Int{})
	gob.Register(cadence.Int8(0))
	gob.Register(cadence.Int16(0))
	gob.Register(cadence.Int32(0))
	gob.Register(cadence.Int64(0))
	gob.Register(cadence.Int128{})
	gob.Register(cadence.Int256{})
	gob.Register(cadence.UInt{})
	gob.Register(cadence.UInt8(0))
	gob.Register(cadence.UInt16(0))
	gob.Register(cadence.UInt32(0))
	gob.Register(cadence.UInt64(0))
	gob.Register(cadence.UInt128{})
	gob.Register(cadence.UInt256{})
	gob.Register(cadence.Word8(0))
	gob.Register(cadence.Word16(0))
	gob.Register(cadence.Word32(0))
	gob.Register(cadence.Word64(0))
	gob.Register(cadence.Fix64(0))
	gob.Register(cadence.UFix64(0))
	gob.Register(cadence.Array{})
	gob.Register(cadence.Dictionary{})
	gob.Register(cadence.KeyValuePair{})
	gob.Register(cadence.Resource{})
	gob.Register(cadence.Event{})
}

func encodeBlock(block types.Block) ([]byte, error) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(&block); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func decodeBlock(block *types.Block, from []byte) error {
	return gob.NewDecoder(bytes.NewBuffer(from)).Decode(block)
}

func encodeCollection(col model.LightCollection) ([]byte, error) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(&col); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func decodeCollection(col *model.LightCollection, from []byte) error {
	return gob.NewDecoder(bytes.NewBuffer(from)).Decode(col)
}

func encodeTransaction(tx flow.Transaction) ([]byte, error) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(&tx); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func decodeTransaction(tx *flow.Transaction, from []byte) error {
	return gob.NewDecoder(bytes.NewBuffer(from)).Decode(tx)
}

func encodeTransactionResult(result flow.TransactionResult) ([]byte, error) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(&result); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func decodeTransactionResult(result *flow.TransactionResult, from []byte) error {
	return gob.NewDecoder(bytes.NewBuffer(from)).Decode(result)
}

func encodeUint64(v uint64) ([]byte, error) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(&v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func decodeUint64(v *uint64, from []byte) error {
	return gob.NewDecoder(bytes.NewBuffer(from)).Decode(v)
}

func encodeLedger(ledger vm.MapLedger) ([]byte, error) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(&ledger); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func decodeLedger(ledger *vm.MapLedger, from []byte) error {
	return gob.NewDecoder(bytes.NewBuffer(from)).Decode(ledger)
}

func encodeEvent(event flow.Event) ([]byte, error) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(&event); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func decodeEvent(event *flow.Event, from []byte) error {
	return gob.NewDecoder(bytes.NewBuffer(from)).Decode(event)
}

func encodeChangelist(clist changelist) ([]byte, error) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(&clist.blocks); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func decodeChangelist(clist *changelist, from []byte) error {
	return gob.NewDecoder(bytes.NewBuffer(from)).Decode(&clist.blocks)
}
