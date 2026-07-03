package app

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	govv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	govv1beta1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1beta1"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// ValidatorProposalDecorator enforces two Ethwei governance rules across BOTH
// the gov v1 and the legacy v1beta1 message APIs (so neither can be bypassed):
//  1. Only active (bonded) validators may submit proposals.
//  2. The NoWithVeto vote option is disabled — for plain AND weighted votes.
type ValidatorProposalDecorator struct {
	sk *stakingkeeper.Keeper
}

func NewValidatorProposalDecorator(sk *stakingkeeper.Keeper) ValidatorProposalDecorator {
	return ValidatorProposalDecorator{sk: sk}
}

// errNoWithVeto is returned for any attempt to cast a NoWithVeto vote.
var errNoWithVeto = sdkerrors.ErrInvalidRequest.Wrap("NoWithVeto is not supported on Ethwei — use No instead")

func (d ValidatorProposalDecorator) AnteHandle(ctx sdk.Context, tx sdk.Tx, simulate bool, next sdk.AnteHandler) (sdk.Context, error) {
	for _, msg := range tx.GetMsgs() {
		switch m := msg.(type) {
		// --- proposal submission: validator-only (both message APIs) ---
		case *govv1.MsgSubmitProposal:
			if err := d.requireBondedValidator(ctx, m.Proposer); err != nil {
				return ctx, err
			}
		case *govv1beta1.MsgSubmitProposal:
			if err := d.requireBondedValidator(ctx, m.Proposer); err != nil {
				return ctx, err
			}

		// --- votes: block NoWithVeto (both APIs, plain + weighted) ---
		case *govv1.MsgVote:
			if m.Option == govv1.VoteOption_VOTE_OPTION_NO_WITH_VETO {
				return ctx, errNoWithVeto
			}
		case *govv1.MsgVoteWeighted:
			for _, o := range m.Options {
				if o.Option == govv1.VoteOption_VOTE_OPTION_NO_WITH_VETO {
					return ctx, errNoWithVeto
				}
			}
		case *govv1beta1.MsgVote:
			if m.Option == govv1beta1.OptionNoWithVeto {
				return ctx, errNoWithVeto
			}
		case *govv1beta1.MsgVoteWeighted:
			for _, o := range m.Options {
				if o.Option == govv1beta1.OptionNoWithVeto {
					return ctx, errNoWithVeto
				}
			}
		}
	}
	return next(ctx, tx, simulate)
}

// requireBondedValidator returns an error unless proposer is the account
// address whose operator key belongs to an active (bonded) validator.
func (d ValidatorProposalDecorator) requireBondedValidator(ctx sdk.Context, proposer string) error {
	addr, err := sdk.AccAddressFromBech32(proposer)
	if err != nil {
		return sdkerrors.ErrInvalidAddress.Wrapf("invalid proposer address: %s", err)
	}
	val, err := d.sk.GetValidator(ctx, sdk.ValAddress(addr))
	if err != nil || val.Status != stakingtypes.Bonded {
		return sdkerrors.ErrUnauthorized.Wrap("only active validators can submit proposals on Ethwei")
	}
	return nil
}
