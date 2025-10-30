//go:build manual
// +build manual

package cli

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/opcm"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

// TestManualVerifyOnSepolia is a manual test for verifying contracts on Sepolia.
//
// To run this test:
//
//	go test -tags manual -v -timeout 30m ./pkg/deployer/integration_test/cli -run TestManualVerifyOnSepolia
//
// Required environment variables:
//   - SEPOLIA_RPC_URL: RPC endpoint for Sepolia
//   - ETHERSCAN_API_KEY: Etherscan API key for verification
//   - CONTRACTS_JSON: Path to a JSON file containing deployed contract addresses
//
// The CONTRACTS_JSON file should contain a map of contract names to addresses (you can pick them up from the superchain registry), e.g.:
//
//	{
//	  "protocolVersionsImplAddress": "0x1234...",
//	  "superchainConfigImplAddress": "0x5678..."
//	}
//
// Example usage:
//
//	export SEPOLIA_RPC_URL="https://sepolia.infura.io/v3/YOUR_KEY"
//	export ETHERSCAN_API_KEY="YOUR_ETHERSCAN_KEY"
//	export CONTRACTS_JSON="./deployed-contracts.json"
//	go test -tags manual -v -timeout 30m ./pkg/deployer/integration_test/cli -run TestManualVerifyOnSepolia
func TestManualVerifyOnSepolia(t *testing.T) {
	sepoliaRPC := os.Getenv("SEPOLIA_RPC_URL")
	etherscanKey := os.Getenv("ETHERSCAN_API_KEY")
	contractsFile := os.Getenv("CONTRACTS_JSON")

	if sepoliaRPC == "" {
		t.Skip("SEPOLIA_RPC_URL not set - skipping manual Sepolia verification test")
	}
	if etherscanKey == "" {
		t.Skip("ETHERSCAN_API_KEY not set - skipping manual Sepolia verification test")
	}
	if contractsFile == "" {
		t.Skip("CONTRACTS_JSON not set - skipping manual Sepolia verification test")
	}

	require.FileExists(t, contractsFile, "Contracts JSON file must exist")

	t.Logf("Running manual verification on Sepolia")
	t.Logf("RPC: %s", sepoliaRPC)
	t.Logf("Contracts file: %s", contractsFile)

	runner := NewCLITestRunner(t)

	output := runner.ExpectSuccess(t, []string{
		"verify",
		"--l1-rpc-url", sepoliaRPC,
		"--input-file", contractsFile,
		"--etherscan-api-key", etherscanKey,
		"--verifier", "etherscan",
		"--artifacts-locator", "embedded",
	}, nil)

	t.Logf("Verification output:\n%s", output)

	verified, skipped, failed, err := parseVerifyOutput(output)
	require.NoError(t, err)

	t.Logf("Verification results: verified=%d, skipped=%d, failed=%d", verified, skipped, failed)

	// At least some contracts should be verified or skipped (already verified)
	require.GreaterOrEqual(t, verified+skipped, 1, "Expected at least one contract to be verified or skipped")

	if failed > 0 {
		t.Logf("WARNING: %d contracts failed verification - check output above for details", failed)
	}
}

// TestManualVerifyOnBlockscout is a manual test for verifying contracts on Blockscout.
//
// To run this test:
//
//	go test -tags manual -v -timeout 30m ./pkg/deployer/integration_test/cli -run TestManualVerifyOnBlockscout
//
// Required environment variables:
//   - L1_RPC_URL: RPC endpoint for the L1 chain
//   - BLOCKSCOUT_URL: Blockscout API URL (e.g., https://eth-sepolia.blockscout.com/api/)
//   - CONTRACTS_JSON: Path to a JSON file containing deployed contract addresses
//
// Example usage:
//
//	export L1_RPC_URL="https://sepolia.infura.io/v3/YOUR_KEY"
//	export BLOCKSCOUT_URL="https://eth-sepolia.blockscout.com/api/"
//	export CONTRACTS_JSON="./deployed-contracts.json"
//	go test -tags manual -v -timeout 30m ./pkg/deployer/integration_test/cli -run TestManualVerifyOnBlockscout
func TestManualVerifyOnBlockscout(t *testing.T) {
	l1RPC := os.Getenv("L1_RPC_URL")
	blockscoutURL := os.Getenv("BLOCKSCOUT_URL")
	contractsFile := os.Getenv("CONTRACTS_JSON")

	if l1RPC == "" {
		t.Skip("L1_RPC_URL not set - skipping manual Blockscout verification test")
	}
	if blockscoutURL == "" {
		t.Skip("BLOCKSCOUT_URL not set - skipping manual Blockscout verification test")
	}
	if contractsFile == "" {
		t.Skip("CONTRACTS_JSON not set - skipping manual Blockscout verification test")
	}

	require.FileExists(t, contractsFile, "Contracts JSON file must exist")

	t.Logf("Running manual verification on Blockscout")
	t.Logf("RPC: %s", l1RPC)
	t.Logf("Blockscout URL: %s", blockscoutURL)
	t.Logf("Contracts file: %s", contractsFile)

	runner := NewCLITestRunner(t)

	output := runner.ExpectSuccess(t, []string{
		"verify",
		"--l1-rpc-url", l1RPC,
		"--input-file", contractsFile,
		"--verifier", "blockscout",
		"--verifier-url", blockscoutURL,
		"--etherscan-api-key", "not-needed-for-blockscout", // Blockscout doesn't require an API key but flag is required
		"--artifacts-locator", "embedded",
	}, nil)

	t.Logf("Verification output:\n%s", output)

	verified, skipped, failed, err := parseVerifyOutput(output)
	require.NoError(t, err)

	t.Logf("Verification results: verified=%d, skipped=%d, failed=%d", verified, skipped, failed)

	require.GreaterOrEqual(t, verified+skipped, 1, "Expected at least one contract to be verified or skipped")

	if failed > 0 {
		t.Logf("WARNING: %d contracts failed verification - check output above for details", failed)
	}
}

// TestManualVerifyExistingDeployment is a helper test that can verify contracts from
// an existing state.json file (e.g., from a previous deployment or apply command).
//
// This is useful for verifying contracts that were deployed but not yet verified.
//
// To run this test:
//
//	go test -tags manual -v -timeout 30m ./pkg/deployer/integration_test/cli -run TestManualVerifyExistingDeployment
//
// Required environment variables:
//   - L1_RPC_URL: RPC endpoint for the L1 chain
//   - ETHERSCAN_API_KEY: Etherscan API key (or omit to use Blockscout)
//   - STATE_FILE: Path to state.json file from deployment
//   - VERIFIER: Optional, "etherscan" (default) or "blockscout"
//   - VERIFIER_URL: Optional, custom verifier URL
//
// Example usage:
//
//	export L1_RPC_URL="https://sepolia.infura.io/v3/YOUR_KEY"
//	export ETHERSCAN_API_KEY="YOUR_KEY"
//	export STATE_FILE="./state.json"
//	go test -tags manual -v -timeout 30m ./pkg/deployer/integration_test/cli -run TestManualVerifyExistingDeployment
func TestManualVerifyExistingDeployment(t *testing.T) {
	l1RPC := os.Getenv("L1_RPC_URL")
	stateFile := os.Getenv("STATE_FILE")
	verifier := os.Getenv("VERIFIER")
	verifierURL := os.Getenv("VERIFIER_URL")
	apiKey := os.Getenv("ETHERSCAN_API_KEY")

	if l1RPC == "" {
		t.Skip("L1_RPC_URL not set - skipping manual verification test")
	}
	if stateFile == "" {
		t.Skip("STATE_FILE not set - skipping manual verification test")
	}

	require.FileExists(t, stateFile, "State file must exist")

	if verifier == "" {
		verifier = "etherscan"
	}

	t.Logf("Running manual verification of existing deployment")
	t.Logf("RPC: %s", l1RPC)
	t.Logf("State file: %s", stateFile)
	t.Logf("Verifier: %s", verifier)

	runner := NewCLITestRunner(t)

	args := []string{
		"verify",
		"--l1-rpc-url", l1RPC,
		"--input-file", stateFile,
		"--verifier", verifier,
		"--artifacts-locator", "embedded",
	}

	if apiKey != "" {
		args = append(args, "--etherscan-api-key", apiKey)
	}

	if verifierURL != "" {
		args = append(args, "--verifier-url", verifierURL)
	}

	output := runner.ExpectSuccess(t, args, nil)

	t.Logf("Verification output:\n%s", output)

	verified, skipped, failed, err := parseVerifyOutput(output)
	require.NoError(t, err)

	t.Logf("Verification results: verified=%d, skipped=%d, failed=%d", verified, skipped, failed)

	if failed > 0 {
		t.Logf("WARNING: %d contracts failed verification - check output above for details", failed)
	}
}

// Helper function to create a sample contracts JSON file for testing
func createSampleContractsFile(t *testing.T, output *opcm.DeploySuperchainOutput) string {
	tmpFile, err := os.CreateTemp(t.TempDir(), "contracts-*.json")
	require.NoError(t, err)
	defer tmpFile.Close()

	data := map[string]common.Address{
		"protocolVersionsImplAddress":  output.ProtocolVersionsImpl,
		"protocolVersionsProxyAddress": output.ProtocolVersionsProxy,
		"superchainConfigImplAddress":  output.SuperchainConfigImpl,
		"superchainConfigProxyAddress": output.SuperchainConfigProxy,
		"proxyAdminAddress":            output.SuperchainProxyAdmin,
	}

	encoder := json.NewEncoder(tmpFile)
	encoder.SetIndent("", "  ")
	require.NoError(t, encoder.Encode(data))

	return tmpFile.Name()
}
