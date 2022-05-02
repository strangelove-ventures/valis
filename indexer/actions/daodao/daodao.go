package daodao

import (
	"context"
	"encoding/hex"
	"time"

	cosmwasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/strangelove-ventures/valis/indexer"
	coretypes "github.com/tendermint/tendermint/rpc/core/types"
	"go.uber.org/zap"
)

// BlockActionName is used for configuring block actions via the config file,
// these names are read when starting the indexer for building the list of actions to take at runtime.
const BlockActionName = "daodao"

// DAODAOAction implements the indexer.BlockAction interface, it describes the appropriate actions to take in order
// to parse the DAODAO smart contract data on-chain and index it into a database instance.
type DAODAOAction struct {
	actionName string
	log        *zap.Logger
}

// NewDAODAOAction returns a new DAODAOAction block action to be used by the indexer.
func NewDAODAOAction(log *zap.Logger) *DAODAOAction {
	return &DAODAOAction{
		actionName: BlockActionName,
		log:        log,
	}
}

// Name returns the block action name for identifying this action.
func (a *DAODAOAction) Name() string {
	return a.actionName
}

// MigrateSchema runs schema migrations for the specified models.
func (a *DAODAOAction) MigrateSchema(indexer *indexer.Indexer) error {
	return indexer.DB.AutoMigrate(
		&Code{},
		&Contract{},
		&ExecMsg{},
		&CW20Balance{},
		&CW20Transaction{},
		&Coin{},
		&DAO{},
		&Marketing{},
		&GovToken{},
		&Logo{},
	)
}

// Execute calls the appropriate functions needed for properly parsing data related to the DAODAO smart contracts.
func (a *DAODAOAction) Execute(ctx context.Context, indexer *indexer.Indexer, block *coretypes.ResultBlock) error {
	return a.IndexDAODAOContracts(ctx, indexer, block)
}

// IndexDAODAOContracts parses the tx data in the specified block and indexes the tx data along with
// and DAODAO smart contract related data into a postgres database instance.
func (a *DAODAOAction) IndexDAODAOContracts(ctx context.Context, indexer *indexer.Indexer, block *coretypes.ResultBlock) error {
	for index, tx := range block.Block.Data.Txs {
		// Check if the context has been cancelled on each iteration
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Millisecond * 100):
			// continue
		}

		sdkTx, err := indexer.Client.Codec.TxConfig.TxDecoder()(tx)
		if err != nil {
			a.log.Debug(
				"Failed to decode tx",
				zap.Int64("height", block.Block.Height),
				zap.Int("tx_index", index+1),
				zap.Int("total_txs", len(block.Block.Data.Txs)),
				zap.Error(err),
			)

			// TODO we may want to keep track of txs that fail to be decoded or do something besides log the error
			continue
		}

		// TODO This can fail so results may not end up in db
		// ex. Failed to query tx results. Err: failed to read response body: context deadline exceeded (Client.Timeout or context cancellation while reading body)
		// ex. [Height 2301720] {8/9 txs} - Failed to query tx results. Err: post failed: Post "https://rpc-juno.ecostake.com:443": context deadline exceeded (Client.Timeout exceeded while awaiting headers)
		txRes, err := indexer.Client.QueryTx(ctx, hex.EncodeToString(tx.Hash()), true)
		if err != nil {
			a.log.Debug(
				"Failed to query tx results",
				zap.Int64("height", block.Block.Height),
				zap.Int("tx_index", index+1),
				zap.Int("total_txs", len(block.Block.Data.Txs)),
				zap.Error(err),
			)

			// TODO we may want to retry or keep track of txs that fail to be queried
			continue
		}

		// TODO remove these, just here to kill compiler errors
		_ = txRes

		for msgIndex, msg := range sdkTx.GetMsgs() {
			a.HandleMsgs(indexer, msg, msgIndex, block.Block.Height, tx.Hash())
		}
	}
	return nil
}

func (a *DAODAOAction) HandleMsgs(indexer *indexer.Indexer, msg sdk.Msg, msgIndex int, height int64, hash []byte) {
	switch m := msg.(type) {
	case *cosmwasmtypes.MsgExecuteContract:
		// do te thing
		a.log.Info(
			"RawMsg",
			zap.String("msg", string(m.Msg.Bytes())),
		)
	case *cosmwasmtypes.MsgInstantiateContract:
		// do te thing
		a.log.Info(
			"RawMsg",
			zap.String("msg", string(m.Msg.Bytes())),
		)
	case *cosmwasmtypes.MsgMigrateContract:
		// do te thing
		a.log.Info(
			"RawMsg",
			zap.String("msg", string(m.Msg.Bytes())),
		)
	case *cosmwasmtypes.MsgStoreCode:
		// do te thing
		a.log.Info(
			"RawMsg",
			zap.String("msg", string(m.WASMByteCode)),
		)
	case *cosmwasmtypes.MsgUpdateAdmin:
		// do te thing
		a.log.Info(
			"RawMsg",
			zap.String("msg", m.Contract),
		)
	}
}
