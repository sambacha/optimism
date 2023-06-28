package fault

import (
	"context"
	"math/big"

	"github.com/ethereum-optimism/optimism/op-bindings/bindings"
	"github.com/ethereum-optimism/optimism/op-service/txmgr"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/log"
)

// faultResponder implements the [Responder] interface to send onchain transactions.
type faultResponder struct {
	log log.Logger

	shutdownCtx       context.Context
	cancelShutdownCtx context.CancelFunc

	// Transaction manager and configuration.
	txMgr                  txmgr.TxManager
	MaxPendingTransactions uint64

	// Fault Dispute Game contract.
	fdgAddr               *common.Address
	fdgContractTransactor *bindings.FaultDisputeGameTransactor
	fdgContractCaller     *bindings.FaultDisputeGameCaller
}

// NewFaultResponder returns a new [faultResponder].
func NewFaultResponder(logger log.Logger, txManagr txmgr.TxManager, mpt uint64, fdgAddress *common.Address, client *ethclient.Client) (*faultResponder, error) {
	fdgContractTransactor, err := bindings.NewFaultDisputeGameTransactor(*fdgAddress, client)
	if err != nil {
		return nil, err
	}
	fdgContractCaller, err := bindings.NewFaultDisputeGameCaller(*fdgAddress, client)
	if err != nil {
		return nil, err
	}
	shutdownCtx, cancelShutdownCtx := context.WithCancel(context.Background())
	return &faultResponder{
		log:                    logger,
		shutdownCtx:            shutdownCtx,
		cancelShutdownCtx:      cancelShutdownCtx,
		txMgr:                  txManagr,
		MaxPendingTransactions: mpt,
		fdgAddr:                fdgAddress,
		fdgContractTransactor:  fdgContractTransactor,
		fdgContractCaller:      fdgContractCaller,
	}, nil
}

// Quit shuts down the [faultResponder].
func (r *faultResponder) Quit() {
	r.log.Info("Shutting down fault responder")
	r.cancelShutdownCtx()
}

// buildTxData builds the transaction data for the [faultResponder].
func (r *faultResponder) buildTxData(ctx context.Context, response Claim) (TxData, error) {
	// Parent Claim index in the contract claimData array.
	bigIndex := big.NewInt(int64(response.ParentContractIndex))

	// Build the transaction data using the response claim.
	if response.DefendsParent() {
		txdata, err := r.fdgContractTransactor.Defend(&bind.TransactOpts{}, bigIndex, response.ValueBytes())
		if err != nil {
			return nil, err
		}
		return NewFaultResponderTxData(txdata), nil
	} else {
		txdata, err := r.fdgContractTransactor.Attack(&bind.TransactOpts{}, bigIndex, response.ValueBytes())
		if err != nil {
			return nil, err
		}
		return NewFaultResponderTxData(txdata), nil
	}
}

// Respond takes a [Claim] and executes the response action.
func (r *faultResponder) Respond(ctx context.Context, response Claim) error {
	// Build the transaction data.
	txData, err := r.buildTxData(ctx, response)
	if err != nil {
		return err
	}

	// Send the transaction through the txmgr queue.
	receiptsCh := make(chan txmgr.TxReceipt[TxData])
	queue := txmgr.NewQueue[TxData](ctx, r.txMgr, r.MaxPendingTransactions)
	candidate := txmgr.TxCandidate{
		To:     txData.Destination(),
		TxData: txData.Bytes(),
		// Setting GasLimit to 0 performs gas estimation online through the [txmgr].
		GasLimit: 0,
	}
	queue.Send(txData, candidate, receiptsCh)

	// Block until the transaction receipt is received.
	r.handleReceipts(ctx, receiptsCh)

	return nil
}

// handleReceipts handles the transaction receipts.
// This is a blocking method.
func (r *faultResponder) handleReceipts(ctx context.Context, receiptsCh chan txmgr.TxReceipt[TxData]) {
	for {
		select {
		case rec := <-receiptsCh:
			r.log.Info("responder received receipt", "txHash", rec.Receipt.TxHash, "status", rec.Receipt.Status)
			return
		case <-r.shutdownCtx.Done():
			r.log.Info("closing responder receipt handler")
			return
		}
	}
}

// faultResponderTxData
type faultResponderTxData struct {
	data []byte
	dest *common.Address
}

// NewFaultResponderTxData returns a new [faultResponderTxData].
func NewFaultResponderTxData(tx *types.Transaction) *faultResponderTxData {
	return &faultResponderTxData{
		data: tx.Data(),
		dest: tx.To(),
	}
}

// Bytes returns the transaction data as a byte slice.
func (txdata *faultResponderTxData) Bytes() []byte {
	return txdata.data
}

// Destination returns the destination address of the transaction.
func (txdata *faultResponderTxData) Destination() *common.Address {
	return txdata.dest
}

// TxData is the interface for the data sent in a transaction.
type TxData interface {
	// Bytes returns the transaction data as a byte slice.
	Bytes() []byte
	// Destination returns the destination address of the transaction.
	Destination() *common.Address
}

// TxQueue is an interface for a transaction queue.
type TxQueue interface {
	// Send sends a transaction to the queue.
	Send(txdata TxData, candidate txmgr.TxCandidate, receiptsCh chan txmgr.TxReceipt[TxData])
}
