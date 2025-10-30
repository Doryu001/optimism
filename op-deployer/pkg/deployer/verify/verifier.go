package verify

import (
	"fmt"
	"os"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/urfave/cli/v2"

	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/artifacts"
	"github.com/ethereum-optimism/optimism/op-service/ctxinterrupt"
	oplog "github.com/ethereum-optimism/optimism/op-service/log"
)

func VerifyCLI(cliCtx *cli.Context) error {
	logCfg := oplog.ReadCLIConfig(cliCtx)
	l := oplog.NewLogger(oplog.AppOut(cliCtx), logCfg)
	oplog.SetGlobalLogHandler(l.Handler())

	l1RPCUrl := cliCtx.String(deployer.L1RPCURLFlagName)
	etherscanAPIKey := cliCtx.String(deployer.EtherscanAPIKeyFlagName)
	verifierType := cliCtx.String(deployer.VerifierFlagName)
	verifierUrl := cliCtx.String(deployer.VerifierUrlFlagName)

	if etherscanAPIKey == "" {
		return fmt.Errorf("etherscan-api-key is required")
	}

	inputFile := cliCtx.String(deployer.InputFileFlagName)
	if inputFile == "" {
		return fmt.Errorf("input-file is required")
	}
	contractName := cliCtx.String(deployer.ContractNameFlagName)

	l1ContractsLocator := cliCtx.String(deployer.ArtifactsLocatorFlagName)
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

	cacheDir := deployer.DefaultCacheDir()
	artifactsFS, err := artifacts.Download(ctx, locator, nil, cacheDir)
	if err != nil {
		return fmt.Errorf("failed to get artifacts: %w", err)
	}
	l.Info("Downloaded artifacts")

	artifactsDir, err := artifacts.ExtractArtifactsToTemp()
	if err != nil {
		return fmt.Errorf("failed to extract artifacts: %w", err)
	}
	defer os.RemoveAll(artifactsDir)

	v, err := NewForgeVerifier(ForgeVerifierOpts{
		RpcUrl:       l1RPCUrl,
		VerifierType: verifierType,
		VerifierUrl:  verifierUrl,
		ApiKey:       etherscanAPIKey,
		ChainID:      l1ChainId,
		ArtifactsFS:  artifactsFS,
		ArtifactsDir: artifactsDir,
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
