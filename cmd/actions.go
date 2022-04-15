package cmd

import (
	"fmt"

	"github.com/strangelove-ventures/valis/indexer"
	"github.com/strangelove-ventures/valis/indexer/actions/ibc"
	"go.uber.org/zap"
)

// GetBlockActionByName returns an indexer.BlockAction if there is a configured action matching
// the specified name.
//
// NOTE: New indexer.BlockAction's should be registered here in a case that returns a new struct if
//       the name parameter matches the value returned by BlockAction.Name()
func (c *Config) GetBlockActionByName(log *zap.Logger, name string) (indexer.BlockAction, error) {
	switch name {
	case ibc.BlockActionName:
		return ibc.NewIBCTransfer(log.With(zap.String("block_action", ibc.BlockActionName))), nil
	default:
		return nil, fmt.Errorf("there is no block action configured with the name %s", name)
	}
}
