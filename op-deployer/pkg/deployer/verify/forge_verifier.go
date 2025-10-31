package verify

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/ethereum-optimism/optimism/op-chain-ops/foundry"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/forge"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
)

type ForgeVerifier struct {
	forgeClient  *forge.Client
	rpcUrl       string
	verifierType string
	verifierUrl  string
	apiKey       string
	chainID      uint64
	artifactsFS  foundry.StatDirFs
	artifactsDir string
	logger       log.Logger
}

type ForgeVerifierOpts struct {
	RpcUrl       string
	VerifierType string
	VerifierUrl  string
	ApiKey       string
	ChainID      uint64
	ArtifactsFS  foundry.StatDirFs
	Logger       log.Logger
}

func NewForgeVerifier(opts ForgeVerifierOpts) (*ForgeVerifier, error) {
	if opts.VerifierType != "etherscan" && opts.VerifierType != "blockscout" {
		return nil, fmt.Errorf("unsupported verifier type: %s (must be 'etherscan' or 'blockscout')", opts.VerifierType)
	}

	forgeTomlPath := filepath.Join(fmt.Sprintf("%v", opts.ArtifactsFS), "foundry.toml")
	forgeClient, err := forge.NewStandardClient(forgeTomlPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create forge client: %w", err)
	}

	if opts.VerifierType == "blockscout" && opts.VerifierUrl == "" {
		url, err := getBlockscoutAPIEndpoint(opts.ChainID)
		if err != nil {
			return nil, fmt.Errorf("failed to get verifier URL for chain %d: %w", opts.ChainID, err)
		}
		opts.VerifierUrl = url
	}

	return &ForgeVerifier{
		forgeClient:  forgeClient,
		rpcUrl:       opts.RpcUrl,
		verifierType: opts.VerifierType,
		verifierUrl:  opts.VerifierUrl,
		apiKey:       opts.ApiKey,
		chainID:      opts.ChainID,
		artifactsFS:  opts.ArtifactsFS,
		logger:       opts.Logger,
	}, nil
}

func getBlockscoutAPIEndpoint(l1ChainID uint64) (string, error) {
	switch l1ChainID {
	case 1:
		return "https://eth.blockscout.com/api/", nil
	case 11155111:
		return "https://eth-sepolia.blockscout.com/api/", nil
	default:
		return "", fmt.Errorf("unsupported L1 chain ID: %d", l1ChainID)
	}
}

func getChainName(chainID uint64) (string, error) {
	switch chainID {
	case 1:
		return "mainnet", nil
	case 11155111:
		return "sepolia", nil
	default:
		return "", fmt.Errorf("unsupported chain ID: %d", chainID)
	}
}

func (v *ForgeVerifier) VerifyContract(ctx context.Context, address common.Address, contractName string) error {
	return v.VerifyContractWithConstructorArgs(ctx, address, contractName, "")
}

func (v *ForgeVerifier) VerifyContractWithConstructorArgs(ctx context.Context, address common.Address, contractName string, constructorArgs string) error {
	artifactPath := getArtifactPath(contractName)
	v.logger.Info("Verifying contract with forge",
		"name", contractName,
		"address", address.Hex(),
		"artifactPath", artifactPath,
		"verifier", v.verifierType)

	f, err := v.artifactsFS.Open(artifactPath)
	if err != nil {
		return fmt.Errorf("failed to open artifact %s: %w", artifactPath, err)
	}
	defer f.Close()

	var art foundry.Artifact
	if err := json.NewDecoder(f).Decode(&art); err != nil {
		return fmt.Errorf("failed to decode artifact: %w", err)
	}

	var contractPath string
	for path, name := range art.Metadata.Settings.CompilationTarget {
		contractPath = fmt.Sprintf("%s:%s", path, name)
		break
	}

	if contractPath == "" {
		return fmt.Errorf("failed to find compilation target in artifact")
	}

	compilerVersion := art.Metadata.Compiler.Version
	if compilerVersion == "" {
		return fmt.Errorf("compiler version not found in artifact")
	}

	args := []string{
		address.Hex(),
		contractPath,
		"--compiler-version", compilerVersion,
		"--watch",
		"--guess-constructor-args",
	}

	if v.verifierType == "blockscout" {
		args = append(args, "--chain-id", fmt.Sprintf("%d", v.chainID))
		args = append(args, "--verifier", "blockscout")

		verifierUrl := v.verifierUrl
		if verifierUrl == "" {
			defaultUrl, err := getBlockscoutAPIEndpoint(v.chainID)
			if err != nil {
				return fmt.Errorf("no verifier URL provided and no default available: %w", err)
			}
			verifierUrl = defaultUrl
		}
		args = append(args, "--verifier-url", verifierUrl)
	} else if v.verifierType == "custom" {
		if v.verifierUrl == "" {
			return fmt.Errorf("--verifier-url is required when using custom verifier")
		}
		args = append(args, "--chain-id", fmt.Sprintf("%d", v.chainID))
		args = append(args, "--verifier", "blockscout")
		args = append(args, "--verifier-url", v.verifierUrl)
	} else {
		chainName, err := getChainName(v.chainID)
		if err != nil {
			return fmt.Errorf("failed to get chain name: %w", err)
		}
		args = append(args, "--chain", chainName)
		args = append(args, "--verifier", "etherscan")
	}

	if v.apiKey != "" {
		args = append(args, "--etherscan-api-key", v.apiKey)
	}

	if v.rpcUrl != "" {
		args = append(args, "--rpc-url", v.rpcUrl)
	}

	if constructorArgs != "" {
		args = append(args, "--constructor-args", constructorArgs)
	}

	v.logger.Debug("Running forge verify-contract", "args", strings.Join(args, " "))

	if err := v.forgeClient.VerifyContract(ctx, args...); err != nil {
		errStr := err.Error()
		if strings.Contains(errStr, "Contract source code already verified") ||
			strings.Contains(errStr, "Already Verified") ||
			strings.Contains(errStr, "already verified") ||
			strings.Contains(errStr, "Smart-contract already verified") {
			v.logger.Info("Contract already verified", "name", contractName, "address", address.Hex(), "verifier", v.verifierType)
			return ErrAlreadyVerified
		}

		// Provide helpful context for constructor arg failures
		if strings.Contains(errStr, "constructor") || strings.Contains(errStr, "Constructor") {
			return fmt.Errorf("forge verification failed (likely constructor args mismatch): %w\nNote: Using --guess-constructor-args to extract from creation tx", err)
		}

		return fmt.Errorf("forge verification failed: %w", err)
	}

	v.logger.Info("Contract verified successfully", "name", contractName, "address", address.Hex(), "verifier", v.verifierType)
	return nil
}

var ErrAlreadyVerified = fmt.Errorf("contract already verified")

func (v *ForgeVerifier) VerifyContracts(ctx context.Context, contracts map[string]common.Address) (verified, skipped, failed int) {
	for contractName, addr := range contracts {
		if addr == (common.Address{}) {
			continue
		}

		err := v.VerifyContract(ctx, addr, contractName)
		if err == nil {
			verified++
		} else if err == ErrAlreadyVerified {
			skipped++
		} else {
			v.logger.Error("Failed to verify contract", "name", contractName, "address", addr.Hex(), "error", err)
			failed++
		}
	}

	return verified, skipped, failed
}

func ContractPathToName(contractPath string) string {
	artifactFilename := filepath.Base(contractPath)
	return strings.TrimSuffix(artifactFilename, ".json")
}
