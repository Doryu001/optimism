package cli

import (
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/ethereum-optimism/optimism/op-chain-ops/addresses"
	"github.com/ethereum-optimism/optimism/op-chain-ops/devkeys"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/integration_test/shared"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/opcm"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/standard"
	"github.com/stretchr/testify/require"
)

// TestCLIVerifyBootstrapContracts tests verification of contracts deployed via bootstrap commands
func TestCLIVerifyBootstrapContracts(t *testing.T) {
	l1ChainID := uint64(31337) // Anvil default chain ID
	l1ChainIDBig := big.NewInt(int64(l1ChainID))

	dk, err := devkeys.NewMnemonicDevKeys(devkeys.TestMnemonic)
	require.NoError(t, err)

	superchainProxyAdminOwner := shared.AddrFor(t, dk, devkeys.L1ProxyAdminOwnerRole.Key(l1ChainIDBig))
	protocolVersionsOwner := shared.AddrFor(t, dk, devkeys.SuperchainDeployerKey.Key(l1ChainIDBig))
	guardian := shared.AddrFor(t, dk, devkeys.SuperchainConfigGuardianKey.Key(l1ChainIDBig))
	challenger := shared.AddrFor(t, dk, devkeys.ChallengerRole.Key(l1ChainIDBig))

	t.Run("verify superchain contracts", func(t *testing.T) {
		runner := NewCLITestRunnerWithNetwork(t)
		workDir := runner.GetWorkDir()

		superchainOutputFile := filepath.Join(workDir, "bootstrap_superchain.json")

		runner.ExpectSuccessWithNetwork(t, []string{
			"bootstrap", "superchain",
			"--outfile", superchainOutputFile,
			"--superchain-proxy-admin-owner", superchainProxyAdminOwner.Hex(),
			"--protocol-versions-owner", protocolVersionsOwner.Hex(),
			"--guardian", guardian.Hex(),
		}, nil)

		require.FileExists(t, superchainOutputFile)

		var superchainOutput opcm.DeploySuperchainOutput
		data, err := os.ReadFile(superchainOutputFile)
		require.NoError(t, err)
		err = json.Unmarshal(data, &superchainOutput)
		require.NoError(t, err)
		require.NoError(t, addresses.CheckNoZeroAddresses(superchainOutput))

		mockServer := setupMockBlockscout(t)

		output := runner.ExpectSuccess(t, []string{
			"verify",
			"--l1-rpc-url", runner.l1RPC,
			"--input-file", superchainOutputFile,
			"--etherscan-api-key", "test-key",
			"--verifier", "blockscout",
			"--verifier-url", mockServer + "/api",
			"--artifacts-locator", "embedded",
		}, nil)

		t.Logf("Verify output:\n%s", output)

		assertVerificationSuccess(t, output)
	})

	t.Run("verify implementations contracts", func(t *testing.T) {
		runner := NewCLITestRunnerWithNetwork(t)
		workDir := runner.GetWorkDir()

		superchainOutputFile := filepath.Join(workDir, "bootstrap_superchain.json")
		runner.ExpectSuccessWithNetwork(t, []string{
			"bootstrap", "superchain",
			"--outfile", superchainOutputFile,
			"--superchain-proxy-admin-owner", superchainProxyAdminOwner.Hex(),
			"--protocol-versions-owner", protocolVersionsOwner.Hex(),
			"--guardian", guardian.Hex(),
		}, nil)

		var superchainOutput opcm.DeploySuperchainOutput
		data, err := os.ReadFile(superchainOutputFile)
		require.NoError(t, err)
		err = json.Unmarshal(data, &superchainOutput)
		require.NoError(t, err)

		implsOutputFile := filepath.Join(workDir, "bootstrap_implementations.json")
		runner.ExpectSuccessWithNetwork(t, []string{
			"bootstrap", "implementations",
			"--outfile", implsOutputFile,
			"--mips-version", strconv.Itoa(int(standard.MIPSVersion)),
			"--protocol-versions-proxy", superchainOutput.ProtocolVersionsProxy.Hex(),
			"--superchain-config-proxy", superchainOutput.SuperchainConfigProxy.Hex(),
			"--l1-proxy-admin-owner", superchainProxyAdminOwner.Hex(),
			"--superchain-proxy-admin", superchainOutput.SuperchainProxyAdmin.Hex(),
			"--challenger", challenger.Hex(),
		}, nil)

		require.FileExists(t, implsOutputFile)

		var implsOutput opcm.DeployImplementationsOutput
		data, err = os.ReadFile(implsOutputFile)
		require.NoError(t, err)
		err = json.Unmarshal(data, &implsOutput)
		require.NoError(t, err)

		mockServer := setupMockBlockscout(t)

		output := runner.ExpectSuccess(t, []string{
			"verify",
			"--l1-rpc-url", runner.l1RPC,
			"--input-file", implsOutputFile,
			"--etherscan-api-key", "test-key",
			"--verifier", "blockscout",
			"--verifier-url", mockServer + "/api",
			"--artifacts-locator", "embedded",
		}, nil)

		t.Logf("Verify implementations output:\n%s", output)

		assertVerificationSuccess(t, output)
	})
}

// TestCLIVerifySingleContract tests verification of a single named contract
func TestCLIVerifySingleContract(t *testing.T) {
	l1ChainID := uint64(31337)
	l1ChainIDBig := big.NewInt(int64(l1ChainID))

	dk, err := devkeys.NewMnemonicDevKeys(devkeys.TestMnemonic)
	require.NoError(t, err)

	superchainProxyAdminOwner := shared.AddrFor(t, dk, devkeys.L1ProxyAdminOwnerRole.Key(l1ChainIDBig))
	protocolVersionsOwner := shared.AddrFor(t, dk, devkeys.SuperchainDeployerKey.Key(l1ChainIDBig))
	guardian := shared.AddrFor(t, dk, devkeys.SuperchainConfigGuardianKey.Key(l1ChainIDBig))

	runner := NewCLITestRunnerWithNetwork(t)
	workDir := runner.GetWorkDir()

	superchainOutputFile := filepath.Join(workDir, "bootstrap_superchain.json")
	runner.ExpectSuccessWithNetwork(t, []string{
		"bootstrap", "superchain",
		"--outfile", superchainOutputFile,
		"--superchain-proxy-admin-owner", superchainProxyAdminOwner.Hex(),
		"--protocol-versions-owner", protocolVersionsOwner.Hex(),
		"--guardian", guardian.Hex(),
	}, nil)

	require.FileExists(t, superchainOutputFile)

	mockServer := setupMockBlockscout(t)

	output := runner.ExpectSuccess(t, []string{
		"verify",
		"--l1-rpc-url", runner.l1RPC,
		"--input-file", superchainOutputFile,
		"--contract-name", "proxyAdminAddress",
		"--etherscan-api-key", "test-key",
		"--verifier", "blockscout",
		"--verifier-url", mockServer + "/api",
		"--artifacts-locator", "embedded",
	}, nil)

	t.Logf("Verify single contract output:\n%s", output)

	verified, skipped, failed, err := parseVerifyOutput(output)
	require.NoError(t, err)
	require.Equal(t, 1, verified+skipped, "Exactly one contract should be verified or skipped")
	require.Equal(t, 0, failed, "No contracts should fail verification")
}

// TestCLIVerifyMissingAPIKey tests error handling when API key is missing
func TestCLIVerifyMissingAPIKey(t *testing.T) {
	l1ChainID := uint64(31337)
	l1ChainIDBig := big.NewInt(int64(l1ChainID))

	dk, err := devkeys.NewMnemonicDevKeys(devkeys.TestMnemonic)
	require.NoError(t, err)

	superchainProxyAdminOwner := shared.AddrFor(t, dk, devkeys.L1ProxyAdminOwnerRole.Key(l1ChainIDBig))
	protocolVersionsOwner := shared.AddrFor(t, dk, devkeys.SuperchainDeployerKey.Key(l1ChainIDBig))
	guardian := shared.AddrFor(t, dk, devkeys.SuperchainConfigGuardianKey.Key(l1ChainIDBig))

	runner := NewCLITestRunnerWithNetwork(t)
	workDir := runner.GetWorkDir()

	superchainOutputFile := filepath.Join(workDir, "bootstrap_superchain.json")
	runner.ExpectSuccessWithNetwork(t, []string{
		"bootstrap", "superchain",
		"--outfile", superchainOutputFile,
		"--superchain-proxy-admin-owner", superchainProxyAdminOwner.Hex(),
		"--protocol-versions-owner", protocolVersionsOwner.Hex(),
		"--guardian", guardian.Hex(),
	}, nil)

	require.FileExists(t, superchainOutputFile)

	output := runner.ExpectErrorContains(t, []string{
		"verify",
		"--l1-rpc-url", runner.l1RPC,
		"--input-file", superchainOutputFile,
		"--artifacts-locator", "embedded",
	}, nil, "etherscan-api-key is required")

	t.Logf("Expected error output:\n%s", output)
}

// TestCLIVerifyInvalidFile tests error handling with invalid input file
func TestCLIVerifyInvalidFile(t *testing.T) {
	runner := NewCLITestRunnerWithNetwork(t)
	mockServer := setupMockBlockscout(t)

	output := runner.ExpectErrorContains(t, []string{
		"verify",
		"--l1-rpc-url", runner.l1RPC,
		"--input-file", "nonexistent.json",
		"--etherscan-api-key", "test-key",
		"--verifier", "blockscout",
		"--verifier-url", mockServer + "/api",
		"--artifacts-locator", "embedded",
	}, nil, "input file not found")

	t.Logf("Expected error output:\n%s", output)
}

// setupMockBlockscout creates a mock HTTP server that simulates Blockscout API responses
func setupMockBlockscout(t *testing.T) string {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Handle both GET and POST requests
		var action string
		if r.Method == http.MethodPost {
			// Parse form data for POST requests
			if err := r.ParseForm(); err != nil {
				http.Error(w, "Failed to parse form", http.StatusBadRequest)
				return
			}
			action = r.FormValue("module")
			// Forge uses module=contract&action=verifysourcecode for POST
			if action == "contract" {
				action = r.FormValue("action")
			}
		} else {
			// GET request - use query parameter
			action = r.URL.Query().Get("action")
		}

		switch action {
		case "getabi":
			response := map[string]interface{}{
				"status":  "0",
				"message": "NOTOK",
				"result":  "Contract source code not verified",
			}
			_ = json.NewEncoder(w).Encode(response)

		case "verifysourcecode":
			// Return success for verification submission
			response := map[string]interface{}{
				"status":  "1",
				"message": "OK",
				"result":  "verification_guid_123",
			}
			_ = json.NewEncoder(w).Encode(response)

		case "checkverifystatus":
			// Return verified status
			response := map[string]interface{}{
				"status":  "1",
				"message": "OK",
				"result":  "Pass - Verified",
			}
			_ = json.NewEncoder(w).Encode(response)

		case "getcontractcreation":
			response := map[string]interface{}{
				"status":  "1",
				"message": "OK",
				"result": []map[string]interface{}{
					{
						"contractAddress": "0x0000000000000000000000000000000000000000",
						"contractCreator": "0x0000000000000000000000000000000000000000",
						"txHash":          "0x0000000000000000000000000000000000000000000000000000000000000000",
					},
				},
			}
			_ = json.NewEncoder(w).Encode(response)

		default:
			// For POST requests without recognized action, return success anyway
			// (forge verification format may vary)
			if r.Method == http.MethodPost {
				response := map[string]interface{}{
					"status":  "1",
					"message": "OK",
					"result":  "verification_guid_123",
				}
				_ = json.NewEncoder(w).Encode(response)
			} else {
				http.Error(w, "Unknown action", http.StatusBadRequest)
			}
		}
	}))

	t.Cleanup(server.Close)
	return server.URL
}

// parseVerifyOutput extracts verification stats from command output
func parseVerifyOutput(output string) (verified, skipped, failed int, err error) {
	re := regexp.MustCompile(`numVerified=(\d+)\s+numSkipped=(\d+)\s+numFailed=(\d+)`)
	matches := re.FindStringSubmatch(output)

	if len(matches) != 4 {
		return 0, 0, 0, fmt.Errorf("could not parse verification output")
	}

	verified, err = strconv.Atoi(matches[1])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("failed to parse numVerified: %w", err)
	}

	skipped, err = strconv.Atoi(matches[2])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("failed to parse numSkipped: %w", err)
	}

	failed, err = strconv.Atoi(matches[3])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("failed to parse numFailed: %w", err)
	}

	return verified, skipped, failed, nil
}

// assertVerificationSuccess checks that verification completed without failures
func assertVerificationSuccess(t *testing.T, output string) {
	verified, skipped, failed, err := parseVerifyOutput(output)
	require.NoError(t, err, "Failed to parse verification output")
	require.GreaterOrEqual(t, verified+skipped, 1, "At least one contract should be verified or skipped")
	require.Equal(t, 0, failed, "No contracts should fail verification")

	require.Contains(t, strings.ToLower(output), "complete", "Output should indicate completion")
}
