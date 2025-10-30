package cli

import (
	"encoding/json"
	"math/big"
	"os"
	"path/filepath"
	"testing"

	"github.com/ethereum-optimism/optimism/op-chain-ops/addresses"
	"github.com/ethereum-optimism/optimism/op-chain-ops/devkeys"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/integration_test/shared"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/opcm"
	"github.com/stretchr/testify/require"
)

func TestCLIAutoVerifyWithBootstrap(t *testing.T) {
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

	mockServer := setupMockBlockscout(t)

	output := runner.ExpectSuccessWithNetwork(t, []string{
		"bootstrap", "superchain",
		"--outfile", superchainOutputFile,
		"--superchain-proxy-admin-owner", superchainProxyAdminOwner.Hex(),
		"--protocol-versions-owner", protocolVersionsOwner.Hex(),
		"--guardian", guardian.Hex(),
		"--verify",
		"--verifier", "blockscout",
		"--verifier-url", mockServer + "/api",
		"--etherscan-api-key", "test-key",
	}, nil)

	t.Logf("Bootstrap with auto-verify output:\n%s", output)

	require.FileExists(t, superchainOutputFile)

	var superchainOutput opcm.DeploySuperchainOutput
	data, err := os.ReadFile(superchainOutputFile)
	require.NoError(t, err)
	err = json.Unmarshal(data, &superchainOutput)
	require.NoError(t, err)
	require.NoError(t, addresses.CheckNoZeroAddresses(superchainOutput))

	require.Contains(t, output, "Starting automatic contract verification")
	require.Contains(t, output, "Automatic verification complete")
	require.Contains(t, output, "numVerified")
	require.Contains(t, output, "numFailed=0")
}

