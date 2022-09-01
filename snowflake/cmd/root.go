/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var verbosity int

var logger logr.Logger

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "theia-sf",
	Short: "Manage infrastructure to use Theia with Snowflake backend",
	Long: `Theia can use Snowflake as the data store for flows exported by
the Flow Aggregator in each cluster running Antrea. You need to bring your own
Snowflake account and your own AWS account. This CLI application takes care of
configuring cloud resources in Snowflake and AWS to enable Theia. To get
started:

1. ensure that AWS credentials are available:
   https://aws.github.io/aws-sdk-go-v2/docs/configuring-sdk/#specifying-credentials
2. export your Snowflake credentials as environment variables:
   SNOWFLAKE_ACCOUNT, SNOWFLAKE_USER, SNOWFLAKE_PASSWORD
3. choose an AWS S3 bucket which will be used to store infrastructure state;
   you can create one with "theia-sf create-bucket" if needed
4. choose an AWS KMS key which will be used to encrypt infrastructure state;
   you can create one with "theia-sf create-kms-key" if needed
5. provision infrastructure with:
   "theia-sf onboard --bucket-name <YOUR BUCKET NAME> --key-id <YOUR KMS KEY ID>"`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if verbosity < 0 || verbosity >= 128 {
			return fmt.Errorf("invalid verbosity level %d: it should be >= 0 and < 128", verbosity)
		}
		zc := zap.NewProductionConfig()
		zc.Level = zap.NewAtomicLevelAt(zapcore.Level(-1 * verbosity))
		zc.DisableStacktrace = true
		zapLog, err := zc.Build()
		if err != nil {
			return fmt.Errorf("cannot initialize Zap logger: %w", err)
			panic("Cannot initialize Zap logger")
		}
		logger = zapr.NewLogger(zapLog)
		return nil
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rand.Seed(time.Now().UnixNano())

	rootCmd.PersistentFlags().IntVarP(&verbosity, "verbosity", "v", 0, "log verbosity")
}
