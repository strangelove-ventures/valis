package cmd

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	flagJSON             = "json"
	flagYAML             = "yaml"
	flagConcurrentBlocks = "concurrent-blocks"
	flagDebugAddr        = "debug-addr"
	flagBeginBlock       = "begin-block"
	flagEndBlock         = "end-block"
	flagFile             = "file"
	flagGormLogLevel     = "gorm-log-level"
)

const (
	defaultDebugAddr        = "localhost:49666"
	defaultConcurrentBlocks = 100
	defaultBeginBlock       = 1
	defaultEndBlock         = 0 // This will enable default behavior of using the latest block height
	defaultJSON             = false
	defaultYAML             = false
	defaultGormLogLevel     = "silent"
)

func yamlFlag(v *viper.Viper, cmd *cobra.Command) *cobra.Command {
	cmd.Flags().BoolP(flagYAML, "y", defaultYAML, "returns the response in yaml format")
	if err := v.BindPFlag(flagYAML, cmd.Flags().Lookup(flagYAML)); err != nil {
		panic(err)
	}
	return cmd
}

func jsonFlag(v *viper.Viper, cmd *cobra.Command) *cobra.Command {
	cmd.Flags().BoolP(flagJSON, "j", defaultJSON, "returns the response in json format")
	if err := v.BindPFlag(flagJSON, cmd.Flags().Lookup(flagJSON)); err != nil {
		panic(err)
	}
	return cmd
}

func concurrentBlocksFlag(v *viper.Viper, cmd *cobra.Command) *cobra.Command {
	cmd.Flags().UintP(flagConcurrentBlocks, "b", defaultConcurrentBlocks, "specifies how many blocks to process concurrently")
	if err := v.BindPFlag(flagConcurrentBlocks, cmd.Flags().Lookup(flagConcurrentBlocks)); err != nil {
		panic(err)
	}
	return cmd
}

func debugServerFlags(v *viper.Viper, cmd *cobra.Command) *cobra.Command {
	cmd.Flags().String(flagDebugAddr, defaultDebugAddr, "address to use for debug server. Set empty to disable debug server.")
	if err := v.BindPFlag(flagDebugAddr, cmd.Flags().Lookup(flagDebugAddr)); err != nil {
		panic(err)
	}
	return cmd
}

func beginBlockFlag(v *viper.Viper, cmd *cobra.Command) *cobra.Command {
	cmd.Flags().Int64P(flagBeginBlock, "s", defaultBeginBlock, "block height to start indexing from")
	if err := v.BindPFlag(flagBeginBlock, cmd.Flags().Lookup(flagBeginBlock)); err != nil {
		panic(err)
	}
	return cmd
}

func endBlockFlag(v *viper.Viper, cmd *cobra.Command) *cobra.Command {
	cmd.Flags().Int64P(flagEndBlock, "e", defaultEndBlock, "block height to end indexing at. Default behavior is to use most recent height.")
	if err := v.BindPFlag(flagEndBlock, cmd.Flags().Lookup(flagEndBlock)); err != nil {
		panic(err)
	}
	return cmd
}

func fileFlag(v *viper.Viper, cmd *cobra.Command) *cobra.Command {
	cmd.Flags().StringP(flagFile, "f", "", "fetch json data from specified file")
	if err := v.BindPFlag(flagFile, cmd.Flags().Lookup(flagFile)); err != nil {
		panic(err)
	}
	return cmd
}

func gormLogFlag(v *viper.Viper, cmd *cobra.Command) *cobra.Command {
	cmd.Flags().StringP(flagGormLogLevel, "l", defaultGormLogLevel, "gorm log level. Valid values are silent, error, warn, and info.")
	if err := v.BindPFlag(flagGormLogLevel, cmd.Flags().Lookup(flagGormLogLevel)); err != nil {
		panic(err)
	}
	return cmd
}
