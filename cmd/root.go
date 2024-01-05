/*
Copyright © 2022 Jayson Grace <jayson.e.grace@gmail.com>

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/

package cmd

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/l50/goutils/v2/logging"
	log "github.com/l50/goutils/v2/logging"
	"github.com/l50/goutils/v2/sys"
	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	defaultConfigName  = "config.yaml"
	blueprintKey       = "blueprint"
	containerKey       = "container"
	defaultConfigType  = "yaml"
	logLevelKey        = "log.level"
	packerTemplatesKey = "packer_templates"
	logName            = "warpgate.log"
	logPathKey         = "log.path"
	logToFileKey       = "log.to_file"
)

var (
	blueprint = Blueprint{}
	cfgFile   string
	// Logger is the global logger
	Logger          log.Logger
	warpCfg         string
	packerTemplates = []PackerTemplate{}

	rootCmd = &cobra.Command{
		Use:   "wg",
		Short: "Create new container images with existing provisioning code.",
	}
)

// Execute runs the root cobra command
func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}

func init() {
	cobra.OnInitialize(initConfig)

	home, err := homedir.Dir()
	if err != nil {
		cobra.CheckErr(fmt.Errorf("failed to get home directory: %v", err))
	}
	warpCfg = filepath.Join(home, ".warp", defaultConfigName)

	// Set up Cobra's persistent flags
	pf := rootCmd.PersistentFlags()
	pf.StringVar(&cfgFile, "config", warpCfg, "config file (default is "+warpCfg+")")

	// Initialize global logger
	logCfg := logging.LogConfig{
		Fs:         afero.NewOsFs(),
		LogPath:    filepath.Join(home, ".warp", logName),
		Level:      slog.LevelInfo,
		OutputType: logging.ColorOutput,
		LogToDisk:  true,
	}
	// Logger, err = log.InitLogging(fs, logPath, logLevel, log.ColorOutput, viper.GetBool(logToFileKey))
	Logger, err := logging.InitLogging(&logCfg)
	cobra.CheckErr(err)

	// Set the global logger
	log.GlobalLogger = Logger
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	viper.SetConfigType(defaultConfigType)

	// Determine the configuration file path
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := homedir.Dir()
		cobra.CheckErr(err)
		warpCfg = filepath.Join(home, ".warp", defaultConfigName)
		viper.AddConfigPath(filepath.Dir(warpCfg))
		viper.SetConfigName(defaultConfigName)
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		fmt.Println("No config file found - creating with default values")
		cobra.CheckErr(createConfigFile(warpCfg))

		// Read the newly created configuration file
		cobra.CheckErr(viper.ReadInConfig())
	}

	// Check for required dependencies
	cobra.CheckErr(depCheck())
}

func createConfigFile(cfgPath string) error {
	// Set default log level (options: debug, info, warn, error)
	viper.SetDefault(logLevelKey, "info")

	// Enable or disable logging to file (default: true)
	viper.SetDefault(logToFileKey, true)

	// Default path for the log file, used if log.to_file is true
	if logToFile := viper.GetBool(logToFileKey); logToFile {
		viper.SetDefault(logPathKey, filepath.Join(filepath.Dir(cfgPath), logName))
	}

	// Create directory for config file if it doesn't exist
	cfgDir := filepath.Dir(cfgPath)
	if _, err := os.Stat(cfgDir); os.IsNotExist(err) {
		if err := os.MkdirAll(cfgDir, os.ModePerm); err != nil {
			return fmt.Errorf("failed to create config directory %s: %v", cfgDir, err)
		}
	}

	// Write Viper config to cfgPath
	if err := viper.SafeWriteConfigAs(cfgPath); err != nil {
		if _, ok := err.(viper.ConfigFileAlreadyExistsError); !ok {
			return fmt.Errorf("failed to write config to %s: %v", cfgPath, err)
		}
	}

	return nil
}

func depCheck() error {
	if !sys.CmdExists("packer") || !sys.CmdExists("docker") {
		errMsg := "missing dependencies: please install packer and docker"
		log.L().Error(errMsg)
		return errors.New(errMsg)
	}

	log.L().Debug("All dependencies are satisfied.")

	return nil
}
