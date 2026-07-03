package keeper

import (
	"context"
	"errors"

	"cosmossdk.io/collections"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"

	"ethwei/x/ethwei/types"
)

const (
	// Annual emission: 100,000,000 ETE × 1,000,000 WEI/ETE = 100,000,000,000,000 WEI
	annualEmissionWEI uint64 = 100_000_000_000_000

	// blocksPerYear assumes a 5-second block time: 365.25 × 24 × 3600 / 5 = 6,311,520
	blocksPerYear uint64 = 6_311_520

	// emissionPerBlock = annualEmissionWEI / blocksPerYear ≈ 15,844,043 WEI
	// (≈ 15.84 ETE per block, integer floor)
	emissionPerBlock uint64 = annualEmissionWEI / blocksPerYear

	// maxAdditionalSupplyWEI: 3,000,000,000 ETE × 1,000,000 WEI/ETE = 3,000,000,000,000,000 WEI
	maxAdditionalSupplyWEI uint64 = 3_000_000_000_000_000
)

// MintBlockReward mints the fixed per-block ETE reward and sends it to the
// fee collector so x/distribution can pay out staking rewards. It is a no-op
// once the 3 B ETE staking reserve is exhausted.
func (k Keeper) MintBlockReward(ctx context.Context) error {
	totalMinted, err := k.TotalMinted.Get(ctx)
	if err != nil {
		if !errors.Is(err, collections.ErrNotFound) {
			// A real store error. Fail closed rather than silently resetting the
			// emission accumulator to 0 — doing so would corrupt the hard supply
			// cap and could allow minting beyond the 10 B ETE max supply.
			return err
		}
		// first block — key not yet set
		totalMinted = 0
	}

	if totalMinted >= maxAdditionalSupplyWEI {
		return nil
	}

	// cap the last partial block so we never exceed the hard limit
	mint := emissionPerBlock
	if remaining := maxAdditionalSupplyWEI - totalMinted; remaining < mint {
		mint = remaining
	}

	coins := sdk.NewCoins(sdk.NewInt64Coin(types.Denom, int64(mint)))

	if err := k.mintBankKeeper.MintCoins(ctx, types.ModuleName, coins); err != nil {
		return err
	}

	if err := k.mintBankKeeper.SendCoinsFromModuleToModule(
		ctx, types.ModuleName, authtypes.FeeCollectorName, coins,
	); err != nil {
		return err
	}

	return k.TotalMinted.Set(ctx, totalMinted+mint)
}
