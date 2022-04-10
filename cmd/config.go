package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/spf13/cobra"
	lens "github.com/strangelove-ventures/lens/client"
	"gopkg.in/yaml.v3"
)

func configCmd(a *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "config",
		Aliases: []string{"cfg"},
		Short:   "Manage configuration file",
	}

	cmd.AddCommand(
		configShowCmd(a),
		configInitCmd(),
	)

	return cmd
}

type ChainConfigs []*lens.ChainClientConfig

// Config provides app wide configuration settings.
type Config struct {
	DB           DatabaseConfig `yaml:"database" json:"database"`
	ChainConfigs ChainConfigs   `yaml:"chains" json:"chains"`
	Actions      []string       `yaml:"actions" json:"actions"`
}

// DatabaseConfig represents the connection details for the database.
type DatabaseConfig struct {
	Host     string `yaml:"host" json:"host"`
	Port     int    `yaml:"port" json:"port"`
	User     string `yaml:"user" json:"user"`
	Password string `yaml:"password" json:"password"`
	Name     string `yaml:"db-name" json:"db-name"`
	SSLMode  string `yaml:"ssl-mode" json:"ssl-mode"`
	Driver   string `yaml:"driver" json:"driver"`
}

// configInitCmd initializes an empty config at the location specified via the --home flag.
func configInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "init",
		Aliases: []string{"i"},
		Short:   "Creates a default home directory at path defined by --home",
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s config init --home %s
$ %s cfg i`, appName, defaultHome, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			home, err := cmd.Flags().GetString(flags.FlagHome)
			if err != nil {
				return err
			}

			cfgDir := path.Join(home, "config")
			cfgPath := path.Join(cfgDir, "config.yaml")

			// If the config doesn't exist...
			if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
				// And the config folder doesn't exist...
				if _, err := os.Stat(cfgDir); os.IsNotExist(err) {
					// And the home folder doesn't exist
					if _, err := os.Stat(home); os.IsNotExist(err) {
						// Create the home folder
						if err = os.Mkdir(home, os.ModePerm); err != nil {
							return err
						}
					}
					// Create the home config folder
					if err = os.Mkdir(cfgDir, os.ModePerm); err != nil {
						return err
					}
				}

				// Then create the file...
				f, err := os.Create(cfgPath)
				if err != nil {
					return err
				}
				defer f.Close()

				// And write the default config to that location...
				if _, err = f.Write(defaultConfig()); err != nil {
					return err
				}

				// And return no error...
				return nil
			}

			// Otherwise, the config file exists, and an error is returned...
			return fmt.Errorf("config already exists: %s", cfgPath)
		},
	}
	return cmd
}

// configShowCmd returns the configuration file in json or yaml format.
func configShowCmd(a *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "show",
		Aliases: []string{"s", "list", "l"},
		Short:   "Prints current configuration",
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s config show --home %s
$ %s cfg list`, appName, defaultHome, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			home, err := cmd.Flags().GetString(flags.FlagHome)
			if err != nil {
				return err
			}

			cfgPath := path.Join(home, "config", "config.yaml")
			if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
				if _, err := os.Stat(home); os.IsNotExist(err) {
					return fmt.Errorf("home path does not exist: %s", home)
				}
				return fmt.Errorf("config does not exist: %s", cfgPath)
			}

			jsn, err := cmd.Flags().GetBool(flagJSON)
			if err != nil {
				return err
			}
			yml, err := cmd.Flags().GetBool(flagYAML)
			if err != nil {
				return err
			}
			switch {
			case yml && jsn:
				return fmt.Errorf("can't pass both --json and --yaml, must pick one")
			case jsn:
				out, err := json.Marshal(a.Config)
				if err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(out))
				return nil
			default:
				out, err := yaml.Marshal(a.Config)
				if err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(out))
				return nil
			}
		},
	}

	return yamlFlag(a.Viper, jsonFlag(a.Viper, cmd))
}

// createConfig writes the default config file to disk in the location specified by home.
func createConfig(home string) error {
	cfgPath := path.Join(home, "config.yaml")

	// If the config doesn't exist...
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		// And the config folder doesn't exist...
		// And the home folder doesn't exist
		if _, err := os.Stat(home); os.IsNotExist(err) {
			// Create the home folder
			if err = os.Mkdir(home, os.ModePerm); err != nil {
				return err
			}
		}
	}

	// Then create the file...
	content := defaultConfig()
	if err := os.WriteFile(cfgPath, content, 0600); err != nil {
		return err
	}

	return nil
}

// initConfig reads in the config file and ENV variables if set.
// This is called as a persistent pre-run command of the root command.
func initConfig(cmd *cobra.Command, a *appState) error {
	home, err := cmd.PersistentFlags().GetString(flags.FlagHome)
	if err != nil {
		return err
	}

	cfgPath := path.Join(home, "config", "config.yaml")
	if _, err = os.Stat(cfgPath); err == nil {
		a.Viper.SetConfigFile(cfgPath)
		err = a.Viper.ReadInConfig()
		if err != nil {
			return fmt.Errorf("failed to read in config: %w", err)
		}

		// read the config file bytes
		file, err := os.ReadFile(a.Viper.ConfigFileUsed())
		if err != nil {
			return fmt.Errorf("error reading config file: %w", err)
		}

		// unmarshall them into the struct
		if err = yaml.Unmarshal(file, &a.Config); err != nil {
			return fmt.Errorf("error unmarshalling config: %w", err)
		}

	}

	return nil
}

// AddChainConfig adds a chain config to the applications Config.
func (c *Config) AddChainConfig(chainConfig *lens.ChainClientConfig) (err error) {
	if chainConfig.ChainID == "" {
		return fmt.Errorf("chainConfig ID cannot be empty")
	}
	chn, err := c.GetChainConfig(chainConfig.ChainID)
	if chn != nil || err == nil {
		return fmt.Errorf("chainConfig with ID %s already exists in config", chainConfig.ChainID)
	}
	c.ChainConfigs = append(c.ChainConfigs, chainConfig)
	return nil
}

// GetChainConfig returns the chain configuration for a given chain.
func (c *Config) GetChainConfig(chainID string) (*lens.ChainClientConfig, error) {
	for _, chain := range c.ChainConfigs {
		if chainID == chain.ChainID {
			return chain, nil
		}
	}
	return nil, fmt.Errorf("chain with ID %s is not configured", chainID)
}

// defaultConfig returns the yaml string representation of the default configuration settings.
func defaultConfig() []byte {
	return Config{
		DB: DatabaseConfig{
			Host:     "localhost",
			Port:     5432,
			User:     "anon",
			Password: "password123",
			Name:     "atlas",
			SSLMode:  "disable",
			Driver:   "postgres",
		}}.MustYAML()
}

// ConnectionString returns a string used in connecting to the database,
// the string is created with the database connection details from the Config's DatabaseConfig.
func (c *Config) ConnectionString() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.DB.Host, c.DB.Port, c.DB.User, c.DB.Password, c.DB.Name, c.DB.SSLMode)
}

// MustYAML returns the yaml string representation of the Config,
// and panics on any errors encountered.
func (c Config) MustYAML() []byte {
	out, err := yaml.Marshal(c)
	if err != nil {
		panic(err)
	}
	return out
}
