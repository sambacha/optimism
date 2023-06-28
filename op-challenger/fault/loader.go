package fault

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/log"
)

// ClaimFetcher is a minimal interface around [bindings.FaultDisputeGameCaller].
// This needs to be updated if the [bindings.FaultDisputeGameCaller] interface changes.
type ClaimFetcher interface {
	ClaimData(opts *bind.CallOpts, arg0 *big.Int) (struct {
		ParentIndex uint32
		Countered   bool
		Claim       [32]byte
		Position    *big.Int
		Clock       *big.Int
	}, error)
	ClaimDataLen(opts *bind.CallOpts) (*big.Int, error)
}

// loader pulls in fault dispute game claim data periodically and over subscriptions.
type loader struct {
	log log.Logger

	state Game

	claimFetcher ClaimFetcher
}

// NewLoader creates a new [loader].
func NewLoader(log log.Logger, state Game, claimFetcher ClaimFetcher) *loader {
	return &loader{
		log:          log,
		state:        state,
		claimFetcher: claimFetcher,
	}
}

// fetchClaim fetches a single [Claim] with a hydrated parent.
func (l *loader) fetchClaim(ctx context.Context, arrIndex uint64) (*Claim, error) {
	fetchedClaim, err := l.claimFetcher.ClaimData(&bind.CallOpts{}, new(big.Int).SetUint64(arrIndex))
	if err != nil {
		return nil, err
	}

	claim := Claim{
		ClaimData: ClaimData{
			Value:    fetchedClaim.Claim,
			Position: NewPositionFromGIndex(fetchedClaim.Position.Uint64()),
		},
	}

	if !claim.IsRootPosition() {
		parentIndex := uint64(fetchedClaim.ParentIndex)
		parentClaim, err := l.claimFetcher.ClaimData(&bind.CallOpts{}, new(big.Int).SetUint64(parentIndex))
		if err != nil {
			return nil, err
		}
		claim.Parent = ClaimData{
			Value:    parentClaim.Claim,
			Position: NewPositionFromGIndex(parentClaim.Position.Uint64()),
		}
	}

	return &claim, nil
}

// FetchClaims fetches all claims from the fault dispute game.
func (l *loader) FetchClaims(ctx context.Context) ([]Claim, error) {
	// Get the current claim count.
	claimCount, err := l.claimFetcher.ClaimDataLen(&bind.CallOpts{})
	if err != nil {
		return []Claim{}, err
	}

	// Fetch each claim and build a list.
	claimList := make([]Claim, claimCount.Uint64())
	for i := uint64(0); i < claimCount.Uint64(); i++ {
		claim, err := l.fetchClaim(ctx, i)
		if err != nil {
			return []Claim{}, err
		}
		claimList[i] = *claim
	}

	return claimList, nil
}

// PushClaims pushes a list of claims into the [Game] state.
func (l *loader) PushClaims(ctx context.Context, claims []Claim) error {
	for _, claim := range claims {
		if err := l.state.Put(claim); err != nil {
			return err
		}
	}
	return nil
}
