package verify

import (
	"context"
	"fmt"

	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/artifacts"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/flags"
	"github.com/ethereum/go-ethereum/log"
)

func AutoVerify(ctx context.Context, logger log.Logger, rpcUrl string, chainID uint64, stateFile string, artifactsLocator *artifacts.Locator, verifierType string, verifierUrl string, apiKey string) error {
	if verifierType == "etherscan" && apiKey == "" {
		logger.Warn("Skipping auto-verification: etherscan verifier requires an API key")
		return nil
	}

	logger.Info("Starting automatic contract verification")

	cacheDir := flags.DefaultCacheDir()
	artifactsFS, err := artifacts.Download(ctx, artifactsLocator, nil, cacheDir)
	if err != nil {
		return fmt.Errorf("failed to download artifacts: %w", err)
	}

	v, err := NewForgeVerifier(ForgeVerifierOpts{
		RpcUrl:       rpcUrl,
		VerifierType: verifierType,
		VerifierUrl:  verifierUrl,
		ApiKey:       apiKey,
		ChainID:      chainID,
		ArtifactsFS:  artifactsFS,
		Logger:       logger,
	})
	if err != nil {
		return fmt.Errorf("failed to create verifier: %w", err)
	}

	bundle, err := GetBundleFromFile(stateFile)
	if err != nil {
		return fmt.Errorf("failed to get contract bundle: %w", err)
	}

	numVerified, numSkipped, numFailed := v.VerifyContracts(ctx, bundle)
	logger.Info("Automatic verification complete", "numVerified", numVerified, "numSkipped", numSkipped, "numFailed", numFailed)

	if numFailed > 0 {
		return fmt.Errorf("failed to verify %d contracts", numFailed)
	}

	return nil
}
