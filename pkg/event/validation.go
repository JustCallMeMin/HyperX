package event

import (
	"context"
)

const MaxEventChainDepth = 3

type ChainDepthChecker interface {
	GetEventDepth(ctx context.Context, eventID string) (int, error)
}

func ValidateEventChain(ctx context.Context, event Event, checker ChainDepthChecker) error {
	if event.Metadata().CausationID == nil {
		return nil
	}

	depth, err := checker.GetEventDepth(ctx, event.Metadata().CausationID.String())
	if err != nil {
		return err
	}

	if depth >= MaxEventChainDepth {
		return ErrEventChainTooDeep
	}

	return nil
}

func ValidateMetadata(metadata Metadata) error {
	return metadata.Validate()
}
