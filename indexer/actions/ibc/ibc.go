package ibc

import (
	"context"
	"encoding/hex"
	"fmt"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	transfertypes "github.com/cosmos/ibc-go/v2/modules/apps/transfer/types"
	channeltypes "github.com/cosmos/ibc-go/v2/modules/core/04-channel/types"
	"github.com/jackc/pgtype"
	"github.com/strangelove-ventures/valis/indexer"
	coretypes "github.com/tendermint/tendermint/rpc/core/types"
	"go.uber.org/zap"
)

// BlockActionName is used for configuring block actions via the config file,
// these names are read when starting the indexer for building the list of actions to take at runtime.
const BlockActionName = "ics20_transfers"

// IBCTransferAction implements the indexer.BlockAction interface, it describes the appropriate actions to take in order
// to parse the ics-20 transfer data on-chain and index it into a database instance.
type IBCTransferAction struct {
	actionName string
	log        *zap.Logger
}

// NewIBCTransfer returns a new IBCTransferAction block action to be used by the indexer.
func NewIBCTransfer(log *zap.Logger) *IBCTransferAction {
	return &IBCTransferAction{
		actionName: BlockActionName,
		log:        log,
	}
}

// Name returns the block action name for identifying this action.
func (a *IBCTransferAction) Name() string {
	return a.actionName
}

// MigrateSchema runs schema migrations for the specified models.
func (a *IBCTransferAction) MigrateSchema(indexer *indexer.Indexer) error {
	return indexer.DB.AutoMigrate(
		&Tx{},
		&MsgTransfer{},
		&MsgRecvPacket{},
		&MsgAcknowledgement{},
		&MsgTimeout{},
	)
}

// Execute calls the appropriate functions needed for properly parsing data related to IBC fungible token transfers.
func (a *IBCTransferAction) Execute(ctx context.Context, indexer *indexer.Indexer, block *coretypes.ResultBlock) error {
	return a.IndexIBCTransfers(ctx, indexer, block)
}

// IndexIBCTransfers parses the tx data in the specified block and indexes the tx data along with
// any ics-20 Msg related data into a postgres database instance.
func (a *IBCTransferAction) IndexIBCTransfers(ctx context.Context, indexer *indexer.Indexer, block *coretypes.ResultBlock) error {
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
			// TODO application specific txs fail here (e.g. Osmosis Msgs, GDEX swaps, Akash deployments, etc.)
			// We need to use lens to load all the correct AppModuleBasics when initializing the (*ChainClient).Codec
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

		// Set the appropriate fee values if they exist
		fee := sdkTx.(sdk.FeeTx)
		var feeAmount, feeDenom string
		if len(fee.GetFee()) == 0 {
			feeAmount = "0"
			feeDenom = ""
		} else {
			feeAmount = fee.GetFee()[0].Amount.String()
			feeDenom = fee.GetFee()[0].Denom
		}

		dbTx := &Tx{
			Hash:        pgtype.Bytea{},
			Timestamp:   pgtype.Timestamp{},
			ChainID:     indexer.Client.Config.ChainID,
			BlockHeight: block.Block.Height,
			RawLog:      pgtype.JSONB{},
			Code:        int(txRes.TxResult.Code),
			FeeAmount:   feeAmount,
			FeeDenom:    feeDenom,
			GasUsed:     txRes.TxResult.GasUsed,
			GasWanted:   txRes.TxResult.GasWanted,
		}
		if err = dbTx.Hash.Set(tx.Hash()); err != nil {
			a.log.Warn(
				"Failed to set tx hash on Tx model",
				zap.Int64("height", block.Block.Height),
				zap.String("tx_hash", string(tx.Hash())),
				zap.Int("tx_index", index+1),
				zap.Int("total_txs", len(block.Block.Data.Txs)),
				zap.Error(err),
			)
			continue
		}
		if err = dbTx.Timestamp.Set(block.Block.Time); err != nil {
			a.log.Warn(
				"Failed to set block time on Tx model",
				zap.Int64("height", block.Block.Height),
				zap.String("tx_hash", string(tx.Hash())),
				zap.Time("block_time", block.Block.Time),
				zap.Int("tx_index", index+1),
				zap.Int("total_txs", len(block.Block.Data.Txs)),
				zap.Error(err),
			)
			continue
		}

		// If the TxResult contains errors build a valid JSON string with the error message
		rawLog := txRes.TxResult.Log
		if txRes.TxResult.Code > 0 {
			rawLog = fmt.Sprintf("{\"error\":\"%s\"}", txRes.TxResult.Log)
		}

		if err = dbTx.RawLog.Set(rawLog); err != nil {
			a.log.Warn(
				"Failed to set raw log on Tx model",
				zap.Int64("height", block.Block.Height),
				zap.String("tx_hash", string(tx.Hash())),
				zap.String("raw_log", rawLog),
				zap.Int("tx_index", index+1),
				zap.Int("total_txs", len(block.Block.Data.Txs)),
				zap.Error(err),
			)
			continue
		}

		result := indexer.DB.Create(dbTx)
		a.LogTxInsertion(result.Error, index, len(sdkTx.GetMsgs()), len(block.Block.Data.Txs), block.Block.Height)

		// Parse the msgs in the tx
		for msgIndex, msg := range sdkTx.GetMsgs() {
			a.HandleIBCMsg(indexer, msg, msgIndex, block.Block.Height, tx.Hash())
		}
	}
	return nil
}

// LogTxInsertion appropriately logs a successful or failed attempt to write a tx to the database instance.
func (a *IBCTransferAction) LogTxInsertion(err error, msgIndex, msgCount, txCount int, height int64) {
	if err != nil {
		a.log.Warn(
			"Failed to write tx to database.",
			zap.Int64("height", height),
			zap.Int("tx_index", msgIndex+1),
			zap.Int("tx_count", txCount),
			zap.Int("msg_count", msgCount),
			zap.Error(err),
		)
		return
	}

	a.log.Info(
		"Successfully wrote tx to database.",
		zap.Int64("height", height),
		zap.Int("tx_index", msgIndex+1),
		zap.Int("tx_count", txCount),
		zap.Int("msg_count", msgCount),
	)
}

// HandleIBCMsg checks if the specified sdk.Msg is a MsgTransfer, MsgRecvPacket, MsgTimeout or MsgAcknowledgement
// and if so it attempts to index the msg data into the database instance.
func (a *IBCTransferAction) HandleIBCMsg(indexer *indexer.Indexer, msg sdk.Msg, msgIndex int, height int64, hash []byte) {
	switch m := msg.(type) {
	case *transfertypes.MsgTransfer:
		transfer := &MsgTransfer{
			TxHash:     pgtype.Bytea{},
			MsgIndex:   msgIndex,
			Signer:     m.Sender,
			Sender:     m.Sender,
			Receiver:   m.Receiver,
			Amount:     m.Token.Amount.String(),
			Denom:      m.Token.Denom,
			SrcChannel: m.SourceChannel,
			SrcPort:    m.SourcePort,
			Route:      m.Route(),
		}
		if err := transfer.TxHash.Set(hash); err != nil {
			a.log.Warn(
				"Failed to set tx hash on MsgTransfer model",
				zap.Int64("height", height),
				zap.String("tx_hash", string(hash)),
				zap.Int("msg_index", msgIndex),
				zap.Error(err),
			)
		}

		result := indexer.DB.Create(transfer)
		if result.Error != nil {
			a.log.Warn(
				"Failed to insert MsgTransfer into DB",
				zap.Int64("height", height),
				zap.String("tx_hash", string(hash)),
				zap.Int("msg_index", msgIndex),
				zap.Error(result.Error),
			)
		}
	case *channeltypes.MsgRecvPacket:
		recv := &MsgRecvPacket{
			TxHash:     pgtype.Bytea{},
			MsgIndex:   msgIndex,
			Signer:     m.Signer,
			SrcChannel: m.Packet.SourceChannel,
			DstChannel: m.Packet.DestinationChannel,
			SrcPort:    m.Packet.SourcePort,
			DstPort:    m.Packet.DestinationPort,
		}
		if err := recv.TxHash.Set(hash); err != nil {
			a.log.Warn(
				"Failed to set tx hash on MsgRecvPacket model",
				zap.Int64("height", height),
				zap.String("tx_hash", string(hash)),
				zap.Int("msg_index", msgIndex),
				zap.Error(err),
			)
		}

		result := indexer.DB.Create(recv)
		if result.Error != nil {
			a.log.Warn(
				"Failed to insert MsgRecvPacket into DB",
				zap.Int64("height", height),
				zap.String("tx_hash", string(hash)),
				zap.Int("msg_index", msgIndex),
				zap.Error(result.Error),
			)
		}
	case *channeltypes.MsgTimeout:
		timeout := &MsgTimeout{
			TxHash:     pgtype.Bytea{},
			MsgIndex:   msgIndex,
			Signer:     m.Signer,
			SrcChannel: m.Packet.SourceChannel,
			DstChannel: m.Packet.DestinationChannel,
			SrcPort:    m.Packet.SourcePort,
			DstPort:    m.Packet.DestinationPort,
		}
		if err := timeout.TxHash.Set(hash); err != nil {
			a.log.Warn(
				"Failed to set tx hash on MsgTimeout model",
				zap.Int64("height", height),
				zap.String("tx_hash", string(hash)),
				zap.Int("msg_index", msgIndex),
				zap.Error(err),
			)
		}

		result := indexer.DB.Create(timeout)
		if result.Error != nil {
			a.log.Warn(
				"Failed to insert MsgTimeout into DB",
				zap.Int64("height", height),
				zap.String("hash", string(hash)),
				zap.Int("msg_index", msgIndex),
				zap.Error(result.Error),
			)
		}
	case *channeltypes.MsgAcknowledgement:
		ack := &MsgAcknowledgement{
			TxHash:     pgtype.Bytea{},
			MsgIndex:   msgIndex,
			Signer:     m.Signer,
			SrcChannel: m.Packet.SourceChannel,
			DstChannel: m.Packet.DestinationChannel,
			SrcPort:    m.Packet.SourcePort,
			DstPort:    m.Packet.DestinationPort,
		}
		if err := ack.TxHash.Set(hash); err != nil {
			a.log.Warn(
				"Failed to set tx hash on MsgAcknowledgement model",
				zap.Int64("height", height),
				zap.String("tx_hash", string(hash)),
				zap.Int("msg_index", msgIndex),
				zap.Error(err),
			)
		}

		result := indexer.DB.Create(ack)
		if result.Error != nil {
			a.log.Warn(
				"Failed to insert MsgAcknowledgement into DB",
				zap.Int64("height", height),
				zap.String("hash", string(hash)),
				zap.Int("msg_index", msgIndex),
				zap.Error(result.Error),
			)
		}
	default:
		// TODO: do we need to do anything here?
	}
}
