package verify

import (
	"context"
	"fmt"
	"strings"

	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/artifacts"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/flags"
	"github.com/ethereum/go-ethereum/log"
)

func AutoVerify(ctx context.Context, logger log.Logger, rpcUrl string, chainID uint64, stateFile string, artifactsLocator *artifacts.Locator, verifierTypes string, verifierUrl string, apiKey string) error {
	// Parse comma-separated verifier types
	verifiers := strings.Split(verifierTypes, ",")
	for i := range verifiers {
		verifiers[i] = strings.TrimSpace(verifiers[i])
	}

	// Check if any verifier requires an API key
	needsAPIKey := false
	for _, verifierType := range verifiers {
		if verifierType == "etherscan" {
			needsAPIKey = true
			break
		}
	}

	if needsAPIKey && apiKey == "" {
		logger.Warn("Skipping auto-verification: etherscan verifier requires an API key")
		return nil
	}

	logger.Info("Starting automatic contract verification", "verifiers", verifierTypes)

	// Download artifacts once and reuse for all verifiers
	cacheDir := flags.DefaultCacheDir()
	artifactsFS, err := artifacts.Download(ctx, artifactsLocator, nil, cacheDir)
	if err != nil {
		return fmt.Errorf("failed to download artifacts: %w", err)
	}

	// Get contract bundle once
	bundle, err := GetBundleFromFile(stateFile)
	if err != nil {
		return fmt.Errorf("failed to get contract bundle: %w", err)
	}

	// Track overall results
	totalVerified := 0
	totalSkipped := 0
	totalFailed := 0
	allErrors := []string{}

	// Verify on each verifier
	for _, verifierType := range verifiers {
		logger.Info("Verifying contracts", "verifier", verifierType)

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
			errMsg := fmt.Sprintf("failed to create %s verifier: %v", verifierType, err)
			logger.Error(errMsg)
			allErrors = append(allErrors, errMsg)
			continue
		}

		numVerified, numSkipped, numFailed := v.VerifyContracts(ctx, bundle)
		logger.Info("Verification complete", "verifier", verifierType, "numVerified", numVerified, "numSkipped", numSkipped, "numFailed", numFailed)

		totalVerified += numVerified
		totalSkipped += numSkipped
		totalFailed += numFailed

		if numFailed > 0 {
			allErrors = append(allErrors, fmt.Sprintf("%s: failed to verify %d contracts", verifierType, numFailed))
		}
	}

	logger.Info("Automatic verification complete across all verifiers", "totalVerified", totalVerified, "totalSkipped", totalSkipped, "totalFailed", totalFailed)

	if len(allErrors) > 0 {
		return fmt.Errorf("verification errors: %s", strings.Join(allErrors, "; "))
	}

	return nil
}
