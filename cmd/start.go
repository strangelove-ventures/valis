package cmd

import (
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/strangelove-ventures/valis/internal/indexdebug"
	"go.uber.org/zap"

	_ "github.com/lib/pq"
	"github.com/spf13/cobra"
	lens "github.com/strangelove-ventures/lens/client"
	"github.com/strangelove-ventures/valis/indexer"
)

// startCmd starts the indexer on the specified chain.
func startCmd(a *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "start [chain-id]",
		Aliases: []string{"st"},
		Short:   "Start the indexer",
		Args:    cobra.ExactArgs(1),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s start
$ %s st`, appName, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			// Determine how many goroutines will be used to process blocks
			concurrentBlocks, err := cmd.Flags().GetUint(flagConcurrentBlocks)
			if err != nil {
				return err
			}
			if concurrentBlocks < 1 {
				return fmt.Errorf("invalid flag value %d, value of --concurrent-blocks must be greater than or equal to 1", concurrentBlocks)
			}

			// Get the chain's config for the chain we are indexing
			chainConfig, err := a.Config.GetChainConfig(args[0])
			if err != nil {
				return err
			}

			// Create client from chain config
			chainConfig.Modules = append([]module.AppModuleBasic{}, lens.ModuleBasics...)
			chainClient, err := lens.NewChainClient(
				a.Log.With(zap.String("chain", chainConfig.ChainID)),
				chainConfig,
				os.Getenv("HOME"),
				cmd.InOrStdin(),
				cmd.OutOrStdout(),
			)
			if err != nil {
				return err
			}

			// Create the database connection
			db, err := indexer.ConnectToDatabase(a.Config.DB.Driver, a.Config.ConnectionString())
			if err != nil {
				return err
			}

			// Create the indexer
			i := indexer.NewIndexer(
				a.Log,
				chainClient,
				db,
			)

			// Start the debug server if necessary
			debugAddr, err := cmd.Flags().GetString(flagDebugAddr)
			if err != nil {
				return err
			}
			if debugAddr == "" {
				a.Log.Info("Skipping debug server due to empty debug address flag")
			} else {
				ln, err := net.Listen("tcp", debugAddr)
				if err != nil {
					a.Log.Error("Failed to listen on debug address. If you have another valis process open, use --" + flagDebugAddr + " to pick a different address.")
					return fmt.Errorf("failed to listen on debug address %q: %w", debugAddr, err)
				}
				log := a.Log.With(zap.String("sys", "debughttp"))
				log.Info("Debug server listening", zap.String("addr", debugAddr))
				indexdebug.StartDebugServer(cmd.Context(), log, ln)
			}

			beginBlock, err := cmd.Flags().GetInt64(flagBeginBlock)
			if err != nil {
				return err
			}

			// if users don't specify an end block,
			// use the latest block height.
			endBlock, err := cmd.Flags().GetInt64(flagEndBlock)
			if err != nil {
				return err
			}
			if endBlock == 0 {
				endBlock, err = i.Client.QueryLatestHeight(ctx)
				if err != nil {
					return err
				}
			}

			var blocks []int64
			for i := beginBlock; i < endBlock; i++ {
				blocks = append(blocks, i)
			}

			var actions []indexer.BlockAction
			for _, name := range a.Config.Actions {
				action, err := a.Config.GetBlockActionByName(a.Log, name)
				if err != nil {
					a.Log.Info("Failed to get block action", zap.String("block_action_name", name))
					continue
				}
				actions = append(actions, action)
			}

			if len(actions) == 0 {
				return fmt.Errorf("no block actions configured, check the actions section of your config")
			}

			// Run the indexer
			if err := i.ForEachBlock(ctx, blocks, actions, concurrentBlocks); err != nil {
				return err
			}

			return nil
		},
	}
	return debugServerFlags(a.Viper, beginBlockFlag(a.Viper, endBlockFlag(a.Viper, concurrentBlocksFlag(a.Viper, cmd))))
}
