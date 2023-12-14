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
	"io"
	"os"
	"path/filepath"

	"github.com/fatih/color"
	goutils "github.com/l50/goutils"
	homedir "github.com/mitchellh/go-homedir"
	log "github.com/sirupsen/logrus"
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
	home            string
	logLevel        string
	logFormat       string
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

	home, err = homedir.Dir()
	if err != nil {
		cobra.CheckErr(err)
	}

	warpCfg = filepath.Join(home, ".warp", defaultConfigName)

	pf := rootCmd.PersistentFlags()
	pf.StringVar(
		&cfgFile, "config", warpCfg, "config file (default is "+warpCfg+")")
	pf.BoolVarP(
		&debug, "debug", "", false, "Show debug messages.")

	// Read debug value if it's set in the config file.
	if err := viper.BindPFlag("debug", pf.Lookup("debug")); err != nil {
		log.WithError(err).Error("failed to bind to debug in the config file")
		cobra.CheckErr(err)
	}

	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

	if err := depCheck(); err != nil {
		log.WithError(err).Error("missing dependencies")
		cobra.CheckErr(err)
	}

	// Create warpDir if it does not already exist
	if _, err := os.Stat(filepath.Dir(warpCfg)); os.IsNotExist(err) {
		log.Infof("Creating default config directory %s.", filepath.Dir(warpCfg))
		if err := os.MkdirAll(filepath.Dir(warpCfg), os.ModePerm); err != nil {
			log.WithError(err).Errorf(color.RedString(
				"failed to create %s: %v", filepath.Dir(warpCfg), err))
			cobra.CheckErr(err)
		}
	}

	// Get input logging configuration parameters -or- set to default values.
	pf.StringVar(
		&logLevel, logLevelKey, "info", "The log level.")
	pf.StringVar(
		&logFormat, logFormatKey, "text", "The log format.")
}

func configLogging() error {
	if debug {
		logLevel = "debug"
	}

	parsedLogLevel, err := log.ParseLevel(logLevel)
	if err != nil {
		log.WithFields(log.Fields{"level": logLevel,
			"fallback": "info"}).Warn("Invalid log level")
	}

	log.SetLevel(parsedLogLevel)

	// Set log format
	switch logFormat {
	case "json":
		log.SetFormatter(&log.JSONFormatter{})
	default:
		log.SetFormatter(&log.TextFormatter{
			TimestampFormat: "2006-01-02 15:04:05",
			FullTimestamp:   true,
			ForceColors:     true,
		})
	}

	warpLog := filepath.Join(filepath.Dir(warpCfg), logName)

	f, err := os.OpenFile(warpLog, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.WithError(err).Errorf(color.RedString(
			"failed to open %s: %v", warpLog, err))
		return err
	}

	// Output to both stdout and the log file
	mw := io.MultiWriter(os.Stdout, f)
	log.SetOutput(mw)

	return nil
}

func depCheck() error {
	if !goutils.CmdExists("packer") || !goutils.CmdExists("docker") {
		errMsg := "missing dependencies: please install packer and docker"
		return errors.New(errMsg)
	}
	return nil
}

func createConfigFile(cfgPath string) error {
	// Set default logging values for config
	viper.SetDefault(logLevelKey, logLevel)
	viper.SetDefault(logFormatKey, logFormat)

	// Write viper config to cfgPath
	if err := viper.WriteConfigAs(cfgPath); err != nil {
		log.WithError(err).Errorf("failed to write config to %s: %v", cfgPath, err)
		return err
	}

	// Read values from newly created config
	if err := viper.ReadInConfig(); err != nil {
		log.WithError(err).Errorf("error reading %s config file", cfgPath)
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

	// Attempt to read config from file. If the file doesn't exist,
	// create and populate it.
	if err := viper.ReadInConfig(); err != nil {
		log.Info("No config file found - creating with default values")
		if err := createConfigFile(warpCfg); err != nil {
			log.WithError(err).Errorf("failed to create config file at %s", warpCfg)
			cobra.CheckErr(err)
		}
	}

	// This needs to happen here since we won't have all
	// of the inputs up until initConfig() is called.
	if err := configLogging(); err != nil {
		log.WithError(err).Error("failed to set up logging")
		cobra.CheckErr(err)
	}

	log.Debug("Using config file: ", viper.ConfigFileUsed())
}
