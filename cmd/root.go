/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/luisdavim/configmapper/pkg/config"
	"github.com/luisdavim/configmapper/pkg/filewatcher"
	"github.com/luisdavim/configmapper/pkg/k8swatcher"
)

func mustBindPFlag(key string, f *pflag.Flag) {
	if err := viper.BindPFlag(key, f); err != nil {
		panic(fmt.Sprintf("viper.BindPFlag(%s) failed: %v", key, err))
	}
}

func New(cfg *config.Config) *cobra.Command {
	// cmd represents the base command when called without any subcommands
	cmd := &cobra.Command{
		Use:   "configmapper",
		Short: "Watch files, ConfigMaps and Secrets",
		Long:  `Watch files, ConfigMaps and Secrets.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			fw, err := filewatcher.New(cfg.FileMap)
			if err != nil {
				return err
			}
			signals := make(chan os.Signal, 1)
			signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
			ctx, cancel := context.WithCancel(context.Background())
			go func() {
				if err = fw.Start(ctx); err != nil {
					signals <- syscall.SIGABRT
				}
			}()
			go func() {
				if err := k8swatcher.Start(ctx, cfg.Watcher); err != nil {
					signals <- syscall.SIGABRT
				}
			}()
			<-signals
			cancel()
			return err
		},
	}

	cmd.Flags().BoolP("watch-configmaps", "", false, "Whether to watch ConfigMaps")
	mustBindPFlag("watcher.configMaps", cmd.Flags().Lookup("watch-configmaps"))

	cmd.Flags().BoolP("watch-secrets", "", false, "Whether to watch secrets")
	mustBindPFlag("watcher.secrets", cmd.Flags().Lookup("watch-secrets"))

	cmd.Flags().StringP("default-path", "p", "/tmp", "Default path where to write the files")
	mustBindPFlag("watcher.defaultPath", cmd.Flags().Lookup("default-path"))

	cmd.Flags().StringP("namespaces", "n", "", "Comma separated list of namespaces to watch (defaults to the Pod's namespace)")
	mustBindPFlag("watcher.namespaces", cmd.Flags().Lookup("namespaces"))

	cmd.Flags().StringP("label-selector", "l", "", "Label selector for configMaps and secrets")
	mustBindPFlag("watcher.labelSelector", cmd.Flags().Lookup("label-selector"))

	return cmd
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	var (
		cfgFile string
		cfg     config.Config
	)

	cobra.OnInitialize(func() {
		// Get the configuration
		var err error
		cfg, err = initConfig(cfgFile)
		if err != nil {
			log.Fatalf("error reading config:  %v", err)
		}
	})

	cmd := New(&cfg)
	cmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file (default is $HOME/.configmapper.yaml)")

	if err := cmd.Execute(); err != nil {
		log.Fatalf("error:  %v", err)
	}
}

// initConfig reads in config file and ENV variables if set.
func initConfig(cfgFile string) (config.Config, error) {
	cfg := config.Config{}
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		if err != nil {
			return cfg, err
		}

		// Search config in home directory with name "configmapper.yaml".
		viper.AddConfigPath(".")
		viper.AddConfigPath(home)
		viper.AddConfigPath("/etc/config")
		viper.SetConfigName("configmapper")
		viper.SetConfigType("yaml")
	}

	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_", ".", "_"))
	viper.AutomaticEnv() // read in environment variables that match the config paths

	// If a config file is found, read it in.
	err := viper.ReadInConfig()
	// TODO: should be errors.Is
	// see: https://github.com/spf13/viper/issues/1139
	if errors.As(err, new(viper.ConfigFileNotFoundError)) {
		err = nil
	}
	if err == nil {
		log.Println("Using config file:", viper.ConfigFileUsed())
		err = viper.Unmarshal(&cfg)
	}

	return cfg, err
}
