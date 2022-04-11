package ibc

import (
	"context"
	"encoding/hex"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	transfertypes "github.com/cosmos/ibc-go/v2/modules/apps/transfer/types"
	channeltypes "github.com/cosmos/ibc-go/v2/modules/core/04-channel/types"
	"github.com/strangelove-ventures/valis/indexer"
	coretypes "github.com/tendermint/tendermint/rpc/core/types"
	"go.uber.org/zap"
)

// blockActionName is used for configuring block actions via the config file,
// these names are read when starting the indexer for building the list of actions to take at runtime.
const blockActionName = "ics20_transfers"

// IBCTransfer implements the indexer.BlockAction interface, it describes the appropriate actions to take in order
// to parse the ics-20 transfer data on-chain and index it into a database instance.
type IBCTransfer struct {
	// TODO use gorm and include the schema in the action as well
	actionName string
	log        *zap.Logger
}

// NewIBCTransfer returns a new IBCTransfer block action to be used by the indexer.
func NewIBCTransfer(log *zap.Logger) *IBCTransfer {
	return &IBCTransfer{
		actionName: blockActionName,
		log:        log.With(zap.String("block_action", blockActionName)),
	}
}

// Name returns the block action name for identifying this action.
func (a *IBCTransfer) Name() string {
	return a.actionName
}

// Execute calls the appropriate functions needed for properly parsing data related to IBC fungible token transfers.
func (a *IBCTransfer) Execute(ctx context.Context, indexer *indexer.Indexer, block *coretypes.ResultBlock) error {
	return a.IndexIBCTransfers(ctx, indexer, block)
}

// IndexIBCTransfers parses the tx data in the specified block and indexes the tx data along with
// any ics-20 Msg related data into a postgres database instance.
func (a *IBCTransfer) IndexIBCTransfers(ctx context.Context, indexer *indexer.Indexer, block *coretypes.ResultBlock) error {
	for index, tx := range block.Block.Data.Txs {
		sdkTx, err := indexer.Client.Codec.TxConfig.TxDecoder()(tx)
		if err != nil {
			// TODO application specific txs fail here (e.g. Osmosis Msgs, GDEX swaps, Akash deployments, etc.)
			// We need to use lens to load all the correct AppModuleBasics when initializing the (*ChainClient).Codec
			// err. tx parse error"
			a.log.Debug("Failed to decode tx",
				zap.Int64("height", block.Block.Height),
				zap.Int("tx_index", index+1),
				zap.Int("total_txs", len(block.Block.Data.Txs)),
				zap.Error(err),
			)

			// TODO we may want to retry or keep track of txs that fail to be decoded
			continue
		}

		// TODO This can fail so results may not end up in db
		// ex. Failed to query tx results. Err: failed to read response body: context deadline exceeded (Client.Timeout or context cancellation while reading body)
		// ex. [Height 2301720] {8/9 txs} - Failed to query tx results. Err: post failed: Post "https://rpc-juno.ecostake.com:443": context deadline exceeded (Client.Timeout exceeded while awaiting headers)
		txRes, err := indexer.Client.QueryTx(ctx, hex.EncodeToString(tx.Hash()), true)
		if err != nil {
			a.log.Debug("Failed to query tx results",
				zap.Int64("height", block.Block.Height),
				zap.Int("tx_index", index+1),
				zap.Int("total_txs", len(block.Block.Data.Txs)),
				zap.Error(err),
			)

			// TODO we may want to retry or keep track of txs that fail to be queried
			continue
		}

		fee := sdkTx.(sdk.FeeTx)
		var feeAmount, feeDenom string
		if len(fee.GetFee()) == 0 {
			feeAmount = "0"
			feeDenom = ""
		} else {
			feeAmount = fee.GetFee()[0].Amount.String()
			feeDenom = fee.GetFee()[0].Denom
		}

		if txRes.TxResult.Code > 0 {
			json := fmt.Sprintf("{\"error\":\"%s\"}", txRes.TxResult.Log)
			err = a.InsertTxRow(indexer, tx.Hash(), json, feeAmount, feeDenom, block.Block.Height, txRes.TxResult.GasUsed,
				txRes.TxResult.GasWanted, block.Block.Time, txRes.TxResult.Code)

			a.LogTxInsertion(err, index, len(sdkTx.GetMsgs()), len(block.Block.Data.Txs), block.Block.Height)
		} else {
			err = a.InsertTxRow(indexer, tx.Hash(), txRes.TxResult.Log, feeAmount, feeDenom, block.Block.Height, txRes.TxResult.GasUsed,
				txRes.TxResult.GasWanted, block.Block.Time, txRes.TxResult.Code)

			a.LogTxInsertion(err, index, len(sdkTx.GetMsgs()), len(block.Block.Data.Txs), block.Block.Height)
		}

		for msgIndex, msg := range sdkTx.GetMsgs() {
			a.HandleIBCMsg(indexer, msg, msgIndex, block.Block.Height, tx.Hash())
		}
	}
	return nil
}

// LogTxInsertion appropriately logs a successful or failed attempt to write a tx to the database instance.
func (a *IBCTransfer) LogTxInsertion(err error, msgIndex, msgs, txs int, height int64) {
	if err != nil {
		a.log.Warn("Failed to write tx to database.",
			zap.Int64("height", height),
			zap.Int("tx_index", msgIndex+1),
			zap.Int("tx_count", txs),
			zap.Int("msg_count", msgs),
			zap.Error(err),
		)
	} else {
		a.log.Info("Successfully wrote tx to database.",
			zap.Int64("height", height),
			zap.Int("tx_index", msgIndex+1),
			zap.Int("tx_count", txs),
			zap.Int("msg_count", msgs),
		)
	}
}

// HandleIBCMsg checks if the specified sdk.Msg is a MsgTransfer, MsgRecvPacket, MsgTimeout or MsgAcknowledgement
// and if so it attempts to index the msg data into the database instance.
func (a *IBCTransfer) HandleIBCMsg(indexer *indexer.Indexer, msg sdk.Msg, msgIndex int, height int64, hash []byte) {
	switch m := msg.(type) {
	case *transfertypes.MsgTransfer:
		err := a.InsertMsgTransferRow(indexer, hash, m.Token.Denom, m.SourceChannel, m.Route(), m.Token.Amount.String(), m.Sender,
			m.Sender, m.Receiver, m.SourcePort, msgIndex)
		if err != nil {
			a.log.Warn("Failed to insert MsgTransfer",
				zap.Int64("height", height),
				zap.String("hash", string(hash)),
				zap.Int("msg_index", msgIndex),
				zap.Error(err),
			)
		}
	case *channeltypes.MsgRecvPacket:
		err := a.InsertMsgRecvPacketRow(indexer, hash, m.Signer, m.Packet.SourceChannel,
			m.Packet.DestinationChannel, m.Packet.SourcePort, m.Packet.DestinationPort, msgIndex)
		if err != nil {
			a.log.Warn("Failed to insert MsgRecvPacket",
				zap.Int64("height", height),
				zap.String("hash", string(hash)),
				zap.Int("msg_index", msgIndex),
				zap.Error(err),
			)
		}
	case *channeltypes.MsgTimeout:
		err := a.InsertMsgTimeoutRow(indexer, hash, m.Signer, m.Packet.SourceChannel,
			m.Packet.DestinationChannel, m.Packet.SourcePort, m.Packet.DestinationPort, msgIndex)
		if err != nil {
			a.log.Warn("Failed to insert MsgTimeout",
				zap.Int64("height", height),
				zap.String("hash", string(hash)),
				zap.Int("msg_index", msgIndex),
				zap.Error(err),
			)
		}
	case *channeltypes.MsgAcknowledgement:
		err := a.InsertMsgAckRow(indexer, hash, m.Signer, m.Packet.SourceChannel,
			m.Packet.DestinationChannel, m.Packet.SourcePort, m.Packet.DestinationPort, msgIndex)
		if err != nil {
			a.log.Warn("Failed to insert MsgAcknowledgement",
				zap.Int64("height", height),
				zap.String("hash", string(hash)),
				zap.Int("msg_index", msgIndex),
				zap.Error(err),
			)
		}
	default:
		// TODO: do we need to do anything here?
	}
}
