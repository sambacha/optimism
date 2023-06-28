package fault

import (
	"fmt"
	"testing"

	"github.com/ethereum-optimism/optimism/op-service/txmgr"
	// "github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

// type mockTxData struct {
// 	data []byte
// }
//
// func (m *mockTxData) Bytes() []byte {
// 	return m.data
// }
//
// func (m *mockTxData) Destination() *common.Address {
// 	return &common.Address{}
// }
//
// type mockTxQueue struct{}
//
// func (m *mockTxQueue) Send(txdata TxData, candidate txmgr.TxCandidate, receiptsCh chan txmgr.TxReceipt[TxData]) {
// 	receiptsCh <- txmgr.TxReceipt[TxData]{}
// }

// TestResponder_sendTransaction_NoData tests the sendTransaction method
// with no transaction data provided by the txdata.
func TestResponder_sendTransaction_NoData(t *testing.T) {
	t.Skip("Responder not tested yet")

	receiptChan := make(chan txmgr.TxReceipt[TxData])
	err := fmt.Errorf("error")
	// go func() {
	// 	err = sendTransaction(&mockTxData{}, &mockTxQueue{}, receiptChan)
	// }()
	<-receiptChan
	require.NoError(t, err)
}
