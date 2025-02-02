package fault

import (
	"context"
	"fmt"

	"github.com/ethereum-optimism/optimism/op-bindings/bindings"
	"github.com/ethereum-optimism/optimism/op-challenger/config"
	"github.com/ethereum-optimism/optimism/op-challenger/fault/alphabet"
	"github.com/ethereum-optimism/optimism/op-challenger/fault/cannon"
	"github.com/ethereum-optimism/optimism/op-challenger/fault/types"
	"github.com/ethereum-optimism/optimism/op-service/txmgr"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
)

type Actor interface {
	Act(ctx context.Context) error
}

type GameInfo interface {
	GetGameStatus(context.Context) (types.GameStatus, error)
	GetClaimCount(context.Context) (uint64, error)
}

type GamePlayer struct {
	agent                   Actor
	agreeWithProposedOutput bool
	caller                  GameInfo
	logger                  log.Logger
	cleanup                 func() error

	completed bool
}

func NewGamePlayer(
	ctx context.Context,
	logger log.Logger,
	cfg *config.Config,
	addr common.Address,
	txMgr txmgr.TxManager,
	client bind.ContractCaller,
) (*GamePlayer, error) {
	logger = logger.New("game", addr)
	contract, err := bindings.NewFaultDisputeGameCaller(addr, client)
	if err != nil {
		return nil, fmt.Errorf("failed to bind the fault dispute game contract: %w", err)
	}

	loader := NewLoader(contract)

	gameDepth, err := loader.FetchGameDepth(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch the game depth: %w", err)
	}

	var provider types.TraceProvider
	var updater types.OracleUpdater
	var cleanup func() error
	switch cfg.TraceType {
	case config.TraceTypeCannon:
		cannonProvider, err := cannon.NewTraceProvider(ctx, logger, cfg, client, addr)
		if err != nil {
			return nil, fmt.Errorf("create cannon trace provider: %w", err)
		}
		cleanup = cannonProvider.Cleanup
		provider = cannonProvider
		updater, err = cannon.NewOracleUpdater(ctx, logger, txMgr, addr, client)
		if err != nil {
			return nil, fmt.Errorf("failed to create the cannon updater: %w", err)
		}
	case config.TraceTypeAlphabet:
		provider = alphabet.NewTraceProvider(cfg.AlphabetTrace, gameDepth)
		cleanup = func() error { return nil }
		updater = alphabet.NewOracleUpdater(logger)
	default:
		return nil, fmt.Errorf("unsupported trace type: %v", cfg.TraceType)
	}

	if err := ValidateAbsolutePrestate(ctx, provider, loader); err != nil {
		return nil, fmt.Errorf("failed to validate absolute prestate: %w", err)
	}

	responder, err := NewFaultResponder(logger, txMgr, addr)
	if err != nil {
		return nil, fmt.Errorf("failed to create the responder: %w", err)
	}

	caller, err := NewFaultCallerFromBindings(addr, client)
	if err != nil {
		return nil, fmt.Errorf("failed to bind the fault contract: %w", err)
	}

	return &GamePlayer{
		agent:                   NewAgent(loader, int(gameDepth), provider, responder, updater, cfg.AgreeWithProposedOutput, logger),
		agreeWithProposedOutput: cfg.AgreeWithProposedOutput,
		caller:                  caller,
		logger:                  logger,
		cleanup:                 cleanup,
	}, nil
}

func (g *GamePlayer) ProgressGame(ctx context.Context) bool {
	if g.completed {
		// Game is already complete so don't try to perform further actions.
		g.logger.Trace("Skipping completed game")
		return true
	}
	g.logger.Trace("Checking if actions are required")
	if err := g.agent.Act(ctx); err != nil {
		g.logger.Error("Error when acting on game", "err", err)
	}
	if status, err := g.caller.GetGameStatus(ctx); err != nil {
		g.logger.Warn("Unable to retrieve game status", "err", err)
	} else {
		g.logGameStatus(ctx, status)
		g.completed = status != types.GameStatusInProgress
		return g.completed
	}
	return false
}

func (g *GamePlayer) logGameStatus(ctx context.Context, status types.GameStatus) {
	if status == types.GameStatusInProgress {
		claimCount, err := g.caller.GetClaimCount(ctx)
		if err != nil {
			g.logger.Error("Failed to get claim count for in progress game", "err", err)
			return
		}
		g.logger.Info("Game info", "claims", claimCount, "status", status)
		return
	}
	var expectedStatus types.GameStatus
	if g.agreeWithProposedOutput {
		expectedStatus = types.GameStatusChallengerWon
	} else {
		expectedStatus = types.GameStatusDefenderWon
	}
	if expectedStatus == status {
		g.logger.Info("Game won", "status", status)
	} else {
		g.logger.Error("Game lost", "status", status)
	}
}

func (g *GamePlayer) Cleanup() error {
	return g.cleanup()
}
