package cli

import (
	"fmt"

	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/bootstrap"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/verify"
	"github.com/ethereum-optimism/optimism/op-service/ctxinterrupt"
	oplog "github.com/ethereum-optimism/optimism/op-service/log"
	"github.com/urfave/cli/v2"
)

func ApplyCLIWithAutoVerify() func(cliCtx *cli.Context) error {
	return func(cliCtx *cli.Context) error {
		if err := deployer.ApplyCLI()(cliCtx); err != nil {
			return err
		}

		if !cliCtx.Bool(deployer.AutoVerifyFlag.Name) {
			return nil
		}

		workdir := cliCtx.String(deployer.WorkdirFlagName)
		stateFile := fmt.Sprintf("%s/state.json", workdir)
		return autoVerifyAfterDeploy(cliCtx, stateFile, deployer.L1RPCURLFlagName)
	}
}

func ImplementationsCLIWithAutoVerify(cliCtx *cli.Context) error {
	if err := bootstrap.ImplementationsCLI(cliCtx); err != nil {
		return err
	}

	if !cliCtx.Bool(deployer.AutoVerifyFlag.Name) {
		return nil
	}

	outfile := cliCtx.String(bootstrap.OutfileFlagName)
	if outfile == "" {
		return nil
	}

	return autoVerifyAfterDeploy(cliCtx, outfile, "l1-rpc-url")
}

func SuperchainCLIWithAutoVerify(cliCtx *cli.Context) error {
	if err := bootstrap.SuperchainCLI(cliCtx); err != nil {
		return err
	}

	if !cliCtx.Bool(deployer.AutoVerifyFlag.Name) {
		return nil
	}

	outfile := cliCtx.String(bootstrap.OutfileFlagName)
	if outfile == "" {
		return nil
	}

	return autoVerifyAfterDeploy(cliCtx, outfile, deployer.L1RPCURLFlagName)
}

func autoVerifyAfterDeploy(cliCtx *cli.Context, stateFile, rpcUrlFlag string) error {
	ctx := ctxinterrupt.WithCancelOnInterrupt(cliCtx.Context)
	l1RPCUrl := cliCtx.String(rpcUrlFlag)

	logCfg := oplog.ReadCLIConfig(cliCtx)
	l := oplog.NewLogger(oplog.AppOut(cliCtx), logCfg)

	chainID, err := deployer.ChainIDFromRPC(ctx, l1RPCUrl)
	if err != nil {
		return fmt.Errorf("failed to get chain ID: %w", err)
	}

	return verify.AutoVerify(
		ctx,
		l,
		l1RPCUrl,
		chainID.Uint64(),
		stateFile,
		cliCtx.String(deployer.VerifierFlagName),
		cliCtx.String(deployer.VerifierUrlFlagName),
		cliCtx.String(deployer.EtherscanAPIKeyFlagName),
	)
}
