package cmd

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/babylonchain/babylon-relayer/bbnrelayer"
	relaydebug "github.com/babylonchain/babylon-relayer/debug"
	"github.com/cosmos/relayer/v2/relayer"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

// updateClientCmd is the command for updating a CZ light client in Babylon
// (adapted from https://github.com/cosmos/relayer/blob/v2.1.2/cmd/tx.go#L232)
func updateClientCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update-client babylon_chain_name cz_chain_name path_name",
		Short: "update IBC client on babylon_chain_name that tracks cz_chain_name with a configured path",
		Long: `Update IBC client on babylon_chain_name that tracks cz_chain_name with a configured path.
Clients are updated by querying headers from cz_chain_name and then sending the
corresponding update-client message to babylon_chain_name.`,
		Args:    withUsage(cobra.ExactArgs(3)),
		Example: strings.TrimSpace(fmt.Sprintf(`$ %s update-client babylon osmosis demo-path`, AppName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			logger, babylonChain, czChain, err := getLoggerAndPathEnds(cmd, args)
			if err != nil {
				return err
			}

			// ensure that key in babylonChain chain exist
			if exists := babylonChain.ChainProvider.KeyExists(babylonChain.ChainProvider.Key()); !exists {
				return fmt.Errorf("key %s not found on babylonChain chain %s", babylonChain.ChainProvider.Key(), babylonChain.ChainID())
			}

			memo, err := cmd.Flags().GetString("memo")
			if err != nil {
				return err
			}
			numRetries, err := cmd.Flags().GetUint("retry")
			if err != nil {
				return err
			}

			prometheusMetrics := relaydebug.NewPrometheusMetrics()
			relayer := bbnrelayer.New(logger, prometheusMetrics)

			return relayer.UpdateClient(cmd.Context(), babylonChain, czChain, memo, numRetries)
		},
	}

	cmd.Flags().String("memo", "", "a memo to include in relayed packets")
	cmd.Flags().Uint("retry", relayer.RtyAttNum, "number of retry attempts for requests")

	return cmd
}

func keepUpdatingClientCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "keep-update-client babylon_chain_name cz_chain_name path_name",
		Short: "keep updating IBC client on babylon_chain_name that tracks cz_chain_name with a configured path",
		Long: `Keep updating IBC client on babylon_chain_name that tracks cz_chain_name with a configured path.
Clients are updated by querying headers from cz_chain_name and then sending the
corresponding update-client message to babylon_chain_name.`,
		Args:    withUsage(cobra.ExactArgs(3)),
		Example: strings.TrimSpace(fmt.Sprintf(`$ %s keep-update-client babylon osmosis demo-path`, AppName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			logger, babylonChain, czChain, err := getLoggerAndPathEnds(cmd, args)
			if err != nil {
				return err
			}

			// ensure that key in babylonChain chain exists
			if exists := babylonChain.ChainProvider.KeyExists(babylonChain.ChainProvider.Key()); !exists {
				return fmt.Errorf("key %s not found on babylonChain chain %s", babylonChain.ChainProvider.Key(), babylonChain.ChainID())
			}

			// retrieve necessary flags
			memo, err := cmd.Flags().GetString("memo")
			if err != nil {
				return err
			}
			interval, err := cmd.Flags().GetDuration("interval")
			if err != nil {
				return err
			}
			// override retry in relayer config
			numRetries, err := cmd.Flags().GetUint("retry")
			if err != nil {
				return err
			}
			relayer.RtyAttNum = numRetries
			relayer.RtyAtt = retry.Attempts(relayer.RtyAttNum)
			relayer.RtyDel = retry.Delay(time.Second)

			// initialise prometheus registry
			metrics := relaydebug.NewPrometheusMetrics()

			// start debug server with prometheus metrics
			debugAddr, err := cmd.Flags().GetString("debug-addr")
			if err != nil {
				return err
			}
			ln, err := net.Listen("tcp", debugAddr)
			if err != nil {
				logger.Error("Failed to listen on debug address. If you have another relayer process open, use --debug-addr to pick a different address.")
				return fmt.Errorf("failed to listen on debug address %q: %w", debugAddr, err)
			}
			debugServerLogger := logger.With(zap.String("sys", "debughttp"))
			debugServerLogger.Info("Debug server listening", zap.String("addr", debugAddr))
			relaydebug.StartDebugServer(cmd.Context(), debugServerLogger, ln, metrics)

			relayer := bbnrelayer.New(logger, metrics)

			return relayer.KeepUpdatingClient(cmd.Context(), babylonChain, czChain, memo, interval, numRetries)
		},
	}

	cmd.Flags().String("memo", "", "a memo to include in relayed packets")
	cmd.Flags().Duration("interval", time.Minute*10, "the interval between two update-client attempts")
	cmd.Flags().Uint("retry", 20, "number of retry attempts for requests")
	cmd.Flags().String("debug-addr", "", "address for the debug server with Prometheus metrics")

	return cmd
}
