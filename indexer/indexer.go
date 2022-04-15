package indexer

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/avast/retry-go/v4"
	coretypes "github.com/tendermint/tendermint/rpc/core/types"
	"golang.org/x/sync/errgroup"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	lens "github.com/strangelove-ventures/lens/client"
	"go.uber.org/zap"
)

// variables used in retry attempts across the indexer codebase
var (
	RtyAttNum = uint(5)
	RtyAtt    = retry.Attempts(RtyAttNum)
	RtyDel    = retry.Delay(time.Millisecond * 400)
	RtyErr    = retry.LastErrorOnly(true)
)

// Indexer is the access point to a chain's client and a database instance.
// Indexer provides utilities for scanning the blocks of a Cosmos blockchain and,
// performing some actions for each block.
type Indexer struct {
	Client *lens.ChainClient
	DB     *gorm.DB

	log *zap.Logger
}

// BlockAction represents a set of actions to be taken, on a per-block basis, as the Indexer processes blocks.
type BlockAction interface {
	Name() string
	MigrateSchema(indexer *Indexer) error
	Execute(ctx context.Context, indexer *Indexer, block *coretypes.ResultBlock) error
}

func NewIndexer(log *zap.Logger, client *lens.ChainClient, db *gorm.DB) *Indexer {
	return &Indexer{
		Client: client,
		DB:     db,
		log:    log.With(zap.String("indexer", fmt.Sprintf("valis_%s_indexer", client.Config.ChainID))),
	}
}

// ForEachBlock specifies what actions should occur for every block being indexed.
// ForEachBlock will process the blocks using concurrentBlocks number of goroutines.
func (i *Indexer) ForEachBlock(ctx context.Context, blocks []int64, actions []BlockAction, concurrentBlocks uint) error {
	var (
		mutex        sync.Mutex
		failedBlocks = make([]int64, 0)
		sem          = make(chan struct{}, concurrentBlocks)
		eg, egCtx    = errgroup.WithContext(ctx)
	)

	i.log.Info(
		"Starting block queries",
		zap.String("chain_id", i.Client.Config.ChainID),
	)

	for _, h := range blocks {
		h := h
		sem <- struct{}{}

		eg.Go(func() error {
			var block *coretypes.ResultBlock

			// Query a block
			if err := retry.Do(func() error {
				var err error
				block, err = i.Client.RPCClient.Block(egCtx, &h)
				return err
			}, retry.Context(egCtx), RtyAtt, RtyDel, RtyErr, retry.DelayType(retry.BackOffDelay), retry.OnRetry(func(n uint, err error) {
				i.log.Info(
					"Failed to get block",
					zap.Int64("height", h),
					zap.Uint("attempt", n),
					zap.Error(err),
				)
			})); err != nil {
				// If we fail to get a block add it to the slice of failed blocks
				func() {
					mutex.Lock()
					defer mutex.Lock()
					failedBlocks = append(failedBlocks, h)
				}()

				<-sem
				return err
			}

			// Execute BlockAction's for every block
			for _, a := range actions {
				if err := a.Execute(egCtx, i, block); err != nil {
					// TODO how to handle actions failing to execute properly
					i.log.Warn(
						"Failed to execute block action properly",
						zap.String("block_action_name", a.Name()),
						zap.Int64("block_height", block.Block.Height),
						zap.Error(err),
					)
				}
			}

			<-sem
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return err
	}

	// Recursively call the function until there are no failed blocks
	if len(failedBlocks) > 0 {
		return i.ForEachBlock(ctx, failedBlocks, actions, concurrentBlocks)
	}
	return nil
}

// ConnectToDatabase attempts to connect to the database using the specified driver and connection string.
// If a connection cannot be established an error is returned. gormSilent will disable gorm logging if true.
func ConnectToDatabase(connString string, gormLogLevel logger.LogLevel) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.New(postgres.Config{
		DSN:                  connString,
		PreferSimpleProtocol: true,
	}), &gorm.Config{
		Logger: logger.Default.LogMode(gormLogLevel),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initalize db session, ensure db server is running & check conn string: %w", err)
	}

	return db, nil
}
