package cmd

import (
	"embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	bp "github.com/cowdogmoo/warpgate/pkg/blueprint"
	"github.com/cowdogmoo/warpgate/pkg/config"
	log "github.com/cowdogmoo/warpgate/pkg/logging"
	packer "github.com/cowdogmoo/warpgate/pkg/packer"
	"github.com/l50/goutils/v2/sys"
	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	defaultConfigDir  = ".warp"
	defaultConfigName = "config"
	defaultConfigType = "yaml"
)

var (
	//go:embed config/*
	configContentsFs embed.FS
	warpConfigDir    string
	warpConfigFile   string
	cfg              config.Config

	blueprint       = bp.Blueprint{}
	packerTemplates = []packer.BlueprintPacker{}

	rootCmd = &cobra.Command{
		Use:   "wg",
		Short: "WarpGate creates new container images with existing provisioning code.",
	}
)

func init() {
	cobra.OnInitialize(initConfig)
	setupRootCmd(rootCmd)
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	viper.SetConfigType(defaultConfigType)
	viper.AutomaticEnv()

	home, err := homedir.Dir()
	checkErr(err, "Failed to get home directory: %v")

	warpConfigDir = filepath.Join(home, defaultConfigDir)
	warpConfigFile = filepath.Join(warpConfigDir, fmt.Sprintf("%s.%s", defaultConfigName, defaultConfigType))

	// Check if the config file exists, if not create the default config file
	if _, err := os.Stat(warpConfigFile); os.IsNotExist(err) {
		fmt.Printf("Config file not found, creating default config file at %s", warpConfigFile)
		createConfig(warpConfigFile)
	}

	viper.SetConfigFile(warpConfigFile)

	if err := viper.ReadInConfig(); err != nil {
		checkErr(err, "Can't read config: %v")
	}

	if err := viper.Unmarshal(&cfg); err != nil {
		checkErr(err, "Failed to unmarshal config: %v")
	}

	err = log.Initialize(warpConfigDir, cfg.Log.Level, cfg.Log.LogPath)
	checkErr(err, "Failed to initialize the logger: %v")

	// Check for required dependencies after initializing the logger
	checkErr(depCheck(), "Dependency check failed")
}

func createConfig(cfgPath string) {
	cfgDir := filepath.Dir(cfgPath)

	// Ensure the configuration directory exists
	if _, err := os.Stat(cfgDir); os.IsNotExist(err) {
		fmt.Printf("Creating config directory %s", cfgDir)
		checkErr(os.MkdirAll(cfgDir, os.ModePerm), "failed to create config directory %s: %v")
	}

	// Write the default config file if it does not exist
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		configFileData, err := configContentsFs.ReadFile(filepath.Join("config", "config.yaml"))
		checkErr(err, "failed to read embedded config: %v")
		checkErr(os.WriteFile(cfgPath, configFileData, 0644), "failed to write config to %s: %v")
		fmt.Printf("Default config file created at %s", cfgPath)
	} else {
		fmt.Printf("Config file already exists at %s", cfgPath)
	}
}

func setupRootCmd(cmd *cobra.Command) {
	cmd.PersistentFlags().StringVar(&warpConfigFile, "config", "", "config file (default is $HOME/.warp/config.yaml)")
	if err := viper.BindPFlag("config", cmd.PersistentFlags().Lookup("config")); err != nil {
		log.Error("Failed to bind the config flag: %v", err)
	}
}

func depCheck() error {
	if !sys.CmdExists("packer") || !sys.CmdExists("docker") {
		errMsg := "missing dependencies: please install packer and docker"
		log.Error(errMsg)
		return errors.New(errMsg)
	}

	log.Debug("All dependencies are satisfied.")

	return nil
}

func checkErr(err error, format string) {
	if err != nil {
		log.Error(format, err)
		os.Exit(1)
	}
}

// Execute runs the root cobra command. It checks for errors and exits
// the program if any are encountered.
func Execute() {
	checkErr(rootCmd.Execute(), "Command execution failed")
}
