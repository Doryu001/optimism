package verify

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ethereum-optimism/optimism/op-chain-ops/foundry"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/artifacts"
	"github.com/ethereum/go-ethereum/log"
)

func AutoVerify(ctx context.Context, logger log.Logger, rpcUrl string, chainID uint64, stateFile string, verifierType string, verifierUrl string, apiKey string) error {
	if apiKey == "" {
		logger.Warn("Skipping automatic verification: no API key provided")
		return nil
	}

	logger.Info("Starting automatic contract verification")

	bundleRoot, err := artifacts.ExtractArtifactsToTemp()
	if err != nil {
		return fmt.Errorf("failed to extract artifacts: %w", err)
	}
	defer os.RemoveAll(bundleRoot)

	artifactsDir := filepath.Join(bundleRoot, "forge-artifacts")
	artifactsFS := foundry.OpenArtifactsDir(artifactsDir)

	v, err := NewForgeVerifier(ForgeVerifierOpts{
		RpcUrl:       rpcUrl,
		VerifierType: verifierType,
		VerifierUrl:  verifierUrl,
		ApiKey:       apiKey,
		ChainID:      chainID,
		ArtifactsFS:  artifactsFS.FS,
		ArtifactsDir: bundleRoot,
		Logger:       logger,
	})
	if err != nil {
		return fmt.Errorf("failed to create verifier: %w", err)
	}

	bundle, err := GetBundleFromFile(stateFile)
	if err != nil {
		return fmt.Errorf("failed to retrieve bundle from state file: %w", err)
	}

	numVerified, numSkipped, numFailed := v.VerifyContracts(ctx, bundle)

	logger.Info("Automatic verification complete",
		"numVerified", numVerified,
		"numSkipped", numSkipped,
		"numFailed", numFailed)

	if numFailed > 0 {
		logger.Warn("Some contracts failed verification", "numFailed", numFailed)
	}

	return nil
}
