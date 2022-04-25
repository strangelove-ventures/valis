package ibc

import (
	"time"

	"github.com/jackc/pgtype"
)

// Tx represents a single tx, which can contain many messages.
type Tx struct {
	Hash        pgtype.Bytea     `gorm:"primaryKey"`
	Timestamp   pgtype.Timestamp `gorm:"not null"`
	ChainID     string           `gorm:"not null"`
	BlockHeight int64            `gorm:"not null"`
	RawLog      pgtype.JSONB     `gorm:"not null"`
	Code        int              `gorm:"not null"`
	FeeAmount   string
	FeeDenom    string
	GasUsed     int64 `gorm:"not null"`
	GasWanted   int64 `gorm:"not null"`

	MsgTransfers        []MsgTransfer        `gorm:"foreignKey:TxHash;references:Hash"`
	MsgRecvPackets      []MsgRecvPacket      `gorm:"foreignKey:TxHash;references:Hash"`
	MsgAcknowledgements []MsgAcknowledgement `gorm:"foreignKey:TxHash;references:Hash"`
	MsgTimeouts         []MsgTimeout         `gorm:"foreignKey:TxHash;references:Hash"`

	CreatedAt time.Time
	UpdatedAt time.Time
}

// MsgTransfer represents an IBC MsgTransfer packet for fungible token transfers.
type MsgTransfer struct {
	TxHash     pgtype.Bytea `gorm:"primaryKey"`
	MsgIndex   int          `gorm:"primaryKey;autoIncrement:false"`
	Signer     string       `gorm:"not null"`
	Sender     string       `gorm:"not null"`
	Receiver   string       `gorm:"not null"`
	Amount     string       `gorm:"not null"`
	Denom      string       `gorm:"not null"`
	SrcChannel string       `gorm:"not null"`
	SrcPort    string       `gorm:"not null"`
	Route      string       `gorm:"not null"`
}

type MsgRecvPacket struct {
	TxHash     pgtype.Bytea `gorm:"primaryKey"`
	MsgIndex   int          `gorm:"primaryKey;autoIncrement:false"`
	Signer     string       `gorm:"not null"`
	SrcChannel string       `gorm:"not null"`
	DstChannel string       `gorm:"not null"`
	SrcPort    string       `gorm:"not null"`
	DstPort    string       `gorm:"not null"`
}

type MsgAcknowledgement struct {
	TxHash     pgtype.Bytea `gorm:"primaryKey"`
	MsgIndex   int          `gorm:"primaryKey;autoIncrement:false"`
	Signer     string       `gorm:"not null"`
	SrcChannel string       `gorm:"not null"`
	DstChannel string       `gorm:"not null"`
	SrcPort    string       `gorm:"not null"`
	DstPort    string       `gorm:"not null"`
}

type MsgTimeout struct {
	TxHash     pgtype.Bytea `gorm:"primaryKey"`
	MsgIndex   int          `gorm:"primaryKey;autoIncrement:false"`
	Signer     string       `gorm:"not null"`
	SrcChannel string       `gorm:"not null"`
	DstChannel string       `gorm:"not null"`
	SrcPort    string       `gorm:"not null"`
	DstPort    string       `gorm:"not null"`
}

/*
func (a *IBCTransferAction) GetLastStoredBlock(indexer *indexer.Indexer, chainId string) (int64, error) {
	var height int64
	if err := indexer.DB.QueryRow("SELECT MAX(block_height) FROM txs WHERE chainid=$1", chainId).Scan(&height); err != nil {
		return 1, err
	}
	return height, nil
}
*/
