package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	lens "github.com/strangelove-ventures/lens/client"
	registry "github.com/strangelove-ventures/lens/client/chain_registry"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

func chainsCmd(a *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "chains",
		Aliases: []string{"ch"},
		Short:   "Manage chain configurations",
	}

	cmd.AddCommand(
		chainsAddCmd(a),
		chainsRegistryList(a),
	)

	return cmd
}

// chainsAddCmd adds a chain's config to the global application config via either
// adding it from a JSON file or querying it from the cosmos chain registry.
// see: https://github.com/cosmos/chain-registry
func chainsAddCmd(a *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "add [[chain-name]]",
		Aliases: []string{"a"},
		Short: "Add a new chain config to the configuration file by fetching chain metadata from \n" +
			"                the chain-registry or passing a file (-f)",
		Args: cobra.MinimumNArgs(0),
		Example: fmt.Sprintf(strings.TrimSpace(
			` $ %s chains add cosmoshub
$ %s chains add cosmoshub osmosis
$ %s chains add --file chain-configs/ibc0.json`), appName, appName, appName),
		RunE: func(cmd *cobra.Command, args []string) error {
			file, err := cmd.Flags().GetString(flagFile)
			if err != nil {
				return err
			}

			// add chain config from a file or the cosmos chain registry
			switch {
			case file != "":
				if err := addChainConfigFromFile(a, file); err != nil {
					return err
				}
			default:
				if err := addChainConfigsFromRegistry(cmd.Context(), a, args); err != nil {
					return err
				}
			}

			return a.OverwriteConfig(a.Config)
		},
	}

	return fileFlag(a.Viper, cmd)
}

// chainsRegistryList queries for the list of all available chains in the cosmos chain registry.
// see: https://github.com/cosmos/chain-registry
func chainsRegistryList(a *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "registry-list",
		Args:    cobra.NoArgs,
		Aliases: []string{"rl"},
		Short:   "List chains available for configuration from the cosmos chain registry",
		Example: fmt.Sprintf("$ %s chains registry-list", appName),
		RunE: func(cmd *cobra.Command, args []string) error {
			jsn, err := cmd.Flags().GetBool(flagJSON)
			if err != nil {
				return err
			}

			yml, err := cmd.Flags().GetBool(flagYAML)
			if err != nil {
				return err
			}

			chains, err := registry.DefaultChainRegistry(a.Log).ListChains(cmd.Context())
			if err != nil {
				return err
			}

			switch {
			case yml && jsn:
				return fmt.Errorf("can't pass both --json and --yaml, must pick one")
			case yml:
				out, err := yaml.Marshal(chains)
				if err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(out))
				return nil
			case jsn:
				out, err := json.Marshal(chains)
				if err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(out))
				return nil
			default:
				for _, chain := range chains {
					fmt.Fprintln(cmd.OutOrStdout(), chain)
				}
			}
			return nil
		},
	}
	return yamlFlag(a.Viper, jsonFlag(a.Viper, cmd))
}

// addChainConfigFromFile reads a JSON-formatted chain client config from the named file
// and adds it to global application config.
func addChainConfigFromFile(a *appState, file string) error {
	if _, err := os.Stat(file); err != nil {
		return err
	}

	byt, err := os.ReadFile(file)
	if err != nil {
		return err
	}

	var config *lens.ChainClientConfig
	if err = json.Unmarshal(byt, &config); err != nil {
		return err
	}

	if err = a.Config.AddChainConfig(config); err != nil {
		return err
	}

	return nil
}

// addChainConfigsFromRegistry attempts to fetch chain config metadata for the specified chains
// from the cosmos chain registry, and if successful adds it to the global application config.
func addChainConfigsFromRegistry(ctx context.Context, a *appState, chains []string) error {
	chainRegistry := registry.DefaultChainRegistry(a.Log)
	allChains, err := chainRegistry.ListChains(ctx)
	if err != nil {
		return err
	}

	for _, chain := range chains {
		found := false
		for _, possibleChain := range allChains {
			if chain == possibleChain {
				found = true
			}

			if !found {
				a.Log.Warn(
					"Unable to find chain",
					zap.String("chain", chain),
					zap.String("source_link", chainRegistry.SourceLink()),
				)
				continue
			}

			chainInfo, err := chainRegistry.GetChain(ctx, chain)
			if err != nil {
				a.Log.Warn(
					"Error retrieving chain",
					zap.String("chain", chain),
					zap.Error(err),
				)
				continue
			}

			chainConfig, err := chainInfo.GetChainConfig(ctx)
			if err != nil {
				a.Log.Warn(
					"Error generating chain config",
					zap.String("chain", chain),
					zap.Error(err),
				)
				continue
			}

			// add to config
			if err = a.Config.AddChainConfig(chainConfig); err != nil {
				a.Log.Warn(
					"Failed to add chain to config",
					zap.String("chain", chain),
					zap.Error(err),
				)
				return err
			}

			// found the correct chain so move on to next chain in chains
			break
		}
	}

	return nil
}
