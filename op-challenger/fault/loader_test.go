package fault

import (
	"context"
	"fmt"
	"math/big"
	"testing"

	"github.com/ethereum-optimism/optimism/op-node/testlog"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/require"
)

var (
	mockClaimDataError = fmt.Errorf("claim data errored")
	mockClaimLenError  = fmt.Errorf("claim len errored")
	mockPutError       = fmt.Errorf("put errored")
)

type mockGameState struct {
	putCalled int
	putErrors bool
}

func (m *mockGameState) Put(claim Claim) error {
	m.putCalled++
	if m.putErrors {
		return mockPutError
	}
	return nil
}

func (m *mockGameState) Claims() []Claim {
	return []Claim{}
}

func (m *mockGameState) IsDuplicate(claim Claim) bool {
	return false
}

type mockClaimFetcher struct {
	claimDataError bool
	claimLenError  bool
}

func (m *mockClaimFetcher) ClaimData(opts *bind.CallOpts, arg0 *big.Int) (struct {
	ParentIndex uint32
	Countered   bool
	Claim       [32]byte
	Position    *big.Int
	Clock       *big.Int
}, error) {
	if m.claimDataError {
		return struct {
			ParentIndex uint32
			Countered   bool
			Claim       [32]byte
			Position    *big.Int
			Clock       *big.Int
		}{}, mockClaimDataError
	}
	return struct {
		ParentIndex uint32
		Countered   bool
		Claim       [32]byte
		Position    *big.Int
		Clock       *big.Int
	}{
		ParentIndex: 0,
		Countered:   false,
		Claim:       [32]byte{},
		Position:    big.NewInt(0),
		Clock:       big.NewInt(0),
	}, nil
}

func (m *mockClaimFetcher) ClaimDataLen(opts *bind.CallOpts) (*big.Int, error) {
	if m.claimLenError {
		return big.NewInt(0), mockClaimLenError
	}
	return big.NewInt(1), nil
}

// TestLoader_PushClaims_Succeeds tests [loader.PushClaims].
func TestLoader_PushClaims_Succeeds(t *testing.T) {
	log := testlog.Logger(t, log.LvlError)
	mockClaimFetcher := &mockClaimFetcher{}
	mockGameState := &mockGameState{}
	loader := NewLoader(log, mockGameState, mockClaimFetcher)
	err := loader.PushClaims(context.Background(), []Claim{
		{},
		{},
		{},
	})
	require.NoError(t, err)
	require.Equal(t, 3, mockGameState.putCalled)
}

// TestLoader_PushClaims_PutErrors tests [loader.PushClaims]
// when the game state [Put] function call errors.
func TestLoader_PushClaims_PutErrors(t *testing.T) {
	log := testlog.Logger(t, log.LvlError)
	mockClaimFetcher := &mockClaimFetcher{}
	mockGameState := &mockGameState{
		putErrors: true,
	}
	loader := NewLoader(log, mockGameState, mockClaimFetcher)
	err := loader.PushClaims(context.Background(), []Claim{
		{},
		{},
		{},
	})
	require.ErrorIs(t, err, mockPutError)
	require.Equal(t, 1, mockGameState.putCalled)
}

// TestLoader_FetchClaims_Succeeds tests [loader.FetchClaims].
func TestLoader_FetchClaims_Succeeds(t *testing.T) {
	log := testlog.Logger(t, log.LvlError)
	mockClaimFetcher := &mockClaimFetcher{}
	loader := NewLoader(log, &mockGameState{}, mockClaimFetcher)
	claims, err := loader.FetchClaims(context.Background())
	require.NoError(t, err)
	require.Equal(t, len(claims), 1)
}

// TestLoader_FetchClaims_ClaimDataErrors tests [loader.FetchClaims]
// when the claim fetcher [ClaimData] function call errors.
func TestLoader_FetchClaims_ClaimDataErrors(t *testing.T) {
	log := testlog.Logger(t, log.LvlError)
	mockClaimFetcher := &mockClaimFetcher{
		claimDataError: true,
	}
	loader := NewLoader(log, &mockGameState{}, mockClaimFetcher)
	claims, err := loader.FetchClaims(context.Background())
	require.ErrorIs(t, err, mockClaimDataError)
	require.Empty(t, claims)
}

// TestLoader_FetchClaims_ClaimLenErrors tests [loader.FetchClaims]
// when the claim fetcher [ClaimDataLen] function call errors.
func TestLoader_FetchClaims_ClaimLenErrors(t *testing.T) {
	log := testlog.Logger(t, log.LvlError)
	mockClaimFetcher := &mockClaimFetcher{
		claimLenError: true,
	}
	loader := NewLoader(log, &mockGameState{}, mockClaimFetcher)
	claims, err := loader.FetchClaims(context.Background())
	require.ErrorIs(t, err, mockClaimLenError)
	require.Empty(t, claims)
}
