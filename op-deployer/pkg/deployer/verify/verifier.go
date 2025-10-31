package verify

import (
	"fmt"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/urfave/cli/v2"

	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/artifacts"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/flags"
	"github.com/ethereum-optimism/optimism/op-service/ctxinterrupt"
	oplog "github.com/ethereum-optimism/optimism/op-service/log"
)

func VerifyCLI(cliCtx *cli.Context) error {
	logCfg := oplog.ReadCLIConfig(cliCtx)
	l := oplog.NewLogger(oplog.AppOut(cliCtx), logCfg)
	oplog.SetGlobalLogHandler(l.Handler())

	l1RPCUrl := cliCtx.String(flags.L1RPCURLFlagName)
	verifierAPIKey := cliCtx.String(flags.VerifierAPIKeyFlagName)
	verifierType := cliCtx.String(flags.VerifierTypeFlagName)
	verifierUrl := cliCtx.String(flags.VerifierUrlFlagName)

	if verifierType == "etherscan" && verifierAPIKey == "" {
		return fmt.Errorf("verifier-api-key is required for etherscan")
	}

	inputFile := cliCtx.String(flags.InputFileFlagName)
	if inputFile == "" {
		return fmt.Errorf("input-file is required")
	}
	contractName := cliCtx.String(flags.ContractNameFlagName)

	l1ContractsLocator := cliCtx.String(flags.ArtifactsLocatorFlagName)
	if l1ContractsLocator == "" {
		return fmt.Errorf("artifacts-locator is required")
	}

	ctx := ctxinterrupt.WithCancelOnInterrupt(cliCtx.Context)

	l1Client, err := ethclient.Dial(l1RPCUrl)
	if err != nil {
		return fmt.Errorf("failed to connect to L1: %w", err)
	}
	defer l1Client.Close()

	chainId, err := l1Client.ChainID(ctx)
	if err != nil {
		return fmt.Errorf("failed to get chain ID: %w", err)
	}
	l1ChainId := chainId.Uint64()

	locator, err := artifacts.NewLocatorFromURL(l1ContractsLocator)
	if err != nil {
		return fmt.Errorf("failed to parse l1 contracts release locator: %w", err)
	}

	cacheDir := flags.DefaultCacheDir()
	artifactsFS, err := artifacts.Download(ctx, locator, nil, cacheDir)
	if err != nil {
		return fmt.Errorf("failed to get artifacts: %w", err)
	}
	l.Info("Downloaded artifacts")

	v, err := NewForgeVerifier(ForgeVerifierOpts{
		RpcUrl:       l1RPCUrl,
		VerifierType: verifierType,
		VerifierUrl:  verifierUrl,
		ApiKey:       verifierAPIKey,
		ChainID:      l1ChainId,
		ArtifactsFS:  artifactsFS,
		Logger:       l,
	})
	if err != nil {
		return fmt.Errorf("failed to create verifier: %w", err)
	}

	bundle, err := GetBundleFromFile(inputFile)
	if err != nil {
		return fmt.Errorf("failed to retrieve bundle: %w", err)
	}

	var numVerified, numSkipped, numFailed int

	if contractName != "" {
		addr, ok := bundle[contractName]
		if !ok {
			return fmt.Errorf("contract %s not found in bundle", contractName)
		}

		err := v.VerifyContract(ctx, addr, contractName)
		if err == nil {
			numVerified++
		} else if err == ErrAlreadyVerified {
			numSkipped++
		} else {
			return fmt.Errorf("failed to verify contract %s: %w", contractName, err)
		}
	} else {
		numVerified, numSkipped, numFailed = v.VerifyContracts(ctx, bundle)
	}

	l.Info("--- COMPLETE ---")
	l.Info("final results", "numVerified", numVerified, "numSkipped", numSkipped, "numFailed", numFailed)
	// May want to return an error here if numFailed > 0
	return nil
}
