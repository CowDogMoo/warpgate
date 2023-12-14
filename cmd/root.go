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

	log "github.com/cowdogmoo/warpgate/pkg/logging"
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
	logFormatKey       = "log.format"
	packerTemplatesKey = "packer_templates"
	logName            = "warpgate.log"
)

var (
	blueprint       = Blueprint{}
	cfgFile         string
	debug           bool
	err             error
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
		fmt.Fprintf(os.Stderr, "Failed to get home directory: %v\n", err)
		cobra.CheckErr(err)
	}

	warpCfg := filepath.Join(home, ".warp", defaultConfigName)
	logDir := filepath.Dir(warpCfg)
	logPath := filepath.Join(logDir, logName)

	// Create log file using CreateLogFile
	fs := afero.NewOsFs()
	_, err = log.CreateLogFile(fs, logDir, logName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create log file: %v\n", err)
		cobra.CheckErr(err)
	}

	// Initialize global logger
	logLevel := slog.LevelInfo
	if debug {
		logLevel = slog.LevelDebug
	}
	if err := log.InitGlobalLogger(logLevel, logPath); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize global logger: %v\n", err)
		cobra.CheckErr(err)
	}

	// Set up Cobra's persistent flags
	pf := rootCmd.PersistentFlags()
	pf.StringVar(&cfgFile, "config", warpCfg, "config file (default is "+warpCfg+")")
	pf.BoolVarP(&debug, "debug", "", false, "Show debug messages.")
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

func createConfigFile(cfgPath string) error {
	// Set default values for config
	viper.SetDefault(logLevelKey, "info")                                       // Default log level
	viper.SetDefault(logFormatKey, "text")                                      // Default log format
	viper.SetDefault("log.path", filepath.Join(filepath.Dir(cfgPath), logName)) // Default log path

	// Create directory for config file if it doesn't exist
	cfgDir := filepath.Dir(cfgPath)
	if _, err := os.Stat(cfgDir); os.IsNotExist(err) {
		if err := os.MkdirAll(cfgDir, os.ModePerm); err != nil {
			log.L().Errorf("Failed to create config directory %s: %v", cfgDir, err)
			return err
		}
	}

	// Write Viper config to cfgPath
	if err := viper.SafeWriteConfigAs(cfgPath); err != nil {
		if _, ok := err.(viper.ConfigFileAlreadyExistsError); !ok {
			log.L().Errorf("failed to write config to %s: %v", cfgPath, err)
			return err
		}
	}

	// Read values from newly created config
	if err := viper.ReadInConfig(); err != nil {
		log.L().Errorf("error reading %s config file", cfgPath)
		return err
	}

	return nil
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	// Set the config file type to YAML
	viper.SetConfigType(defaultConfigType)

	// If a config file is specified via CLI, use that.
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.AddConfigPath(filepath.Dir(warpCfg))
		viper.SetConfigName(defaultConfigName)
	}

	// Get relevant environment variables tied to viper params.
	viper.AutomaticEnv()

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		log.L().Debugf("Using config file: %s", viper.ConfigFileUsed())
	} else {
		log.L().Printf("No config file found - creating with default values")
		if err := createConfigFile(warpCfg); err != nil {
			log.L().Errorf("Error creating config file: %s", err)
			cobra.CheckErr(err)
		}
	}

	// Create warpDir if it does not already exist
	if _, err := os.Stat(filepath.Dir(warpCfg)); os.IsNotExist(err) {
		log.L().Debugf("Creating default config directory %s.", filepath.Dir(warpCfg))
		if err := os.MkdirAll(filepath.Dir(warpCfg), os.ModePerm); err != nil {
			log.L().Errorf("Failed to create %s: %v", filepath.Dir(warpCfg), err)
			cobra.CheckErr(err)
		}
	}

	// Check for required dependencies
	if err := depCheck(); err != nil {
		log.L().Error("Missing dependencies: ", err)
		cobra.CheckErr(err)
	}
}
