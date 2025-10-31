#!/bin/bash
set -e

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}   OP Deployer - Sepolia Deployment & Verification Script${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"
echo ""

# Get script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
OUTPUT_DIR="$REPO_ROOT/.deployer-output"
mkdir -p "$OUTPUT_DIR"

# Check if binary exists, build if not
BINARY="$REPO_ROOT/bin/op-deployer"
if [ ! -f "$BINARY" ]; then
    echo -e "${YELLOW}⚠️  op-deployer binary not found. Building...${NC}"
    cd "$REPO_ROOT"
    mise exec -- go build -o "$BINARY" ./cmd/op-deployer
    echo -e "${GREEN}✓ Binary built successfully${NC}"
    echo ""
fi

# Prompt for deployment type
echo -e "${BLUE}What would you like to deploy?${NC}"
echo "  1) Superchain contracts (recommended for first deployment)"
echo "  2) Implementation contracts (requires existing superchain deployment)"
echo ""
read -p "Enter choice [1-2]: " DEPLOY_TYPE

# Prompt for required inputs
echo ""
echo -e "${BLUE}━━━ Required Inputs ━━━${NC}"
echo ""

# Sepolia RPC URL
echo -e "${YELLOW}Sepolia RPC URL${NC}"
echo "  Examples:"
echo "    - https://sepolia.infura.io/v3/YOUR_KEY"
echo "    - https://eth-sepolia.g.alchemy.com/v2/YOUR_KEY"
echo "    - https://rpc.sepolia.org (public, may be slow)"
echo ""
read -p "Enter Sepolia RPC URL: " L1_RPC_URL

# Private Key
echo ""
echo -e "${YELLOW}Private Key${NC}"
echo "  ⚠️  This account must have Sepolia ETH (~0.1-0.2 ETH recommended)"
echo "  ⚠️  Never use mainnet keys or keys with real funds!"
echo ""
read -sp "Enter private key (hidden): " PRIVATE_KEY
echo ""

# Etherscan API Key
echo ""
echo -e "${YELLOW}Etherscan API Key${NC}"
echo "  Get one free at: https://etherscan.io/myapikey"
echo "  (Optional: press Enter to skip verification)"
echo ""
read -p "Enter Etherscan API key: " ETHERSCAN_API_KEY

# Deployment-specific parameters
if [ "$DEPLOY_TYPE" == "1" ]; then
    echo ""
    echo -e "${BLUE}━━━ Superchain Configuration ━━━${NC}"
    echo ""
    echo -e "${YELLOW}Admin/Owner Addresses${NC}"
    echo "  You can use the same address for all roles for testing"
    echo ""
    
    read -p "Superchain Proxy Admin Owner: " PROXY_ADMIN_OWNER
    read -p "Protocol Versions Owner: " PROTOCOL_VERSIONS_OWNER
    read -p "Guardian Address: " GUARDIAN
    
    OUTPUT_FILE="$OUTPUT_DIR/sepolia-superchain-$(date +%Y%m%d-%H%M%S).json"
    
elif [ "$DEPLOY_TYPE" == "2" ]; then
    echo ""
    echo -e "${BLUE}━━━ Implementation Configuration ━━━${NC}"
    echo ""
    echo -e "${YELLOW}Required: Existing Superchain Addresses${NC}"
    echo "  These should be from a previous superchain deployment"
    echo ""
    
    read -p "Protocol Versions Proxy Address: " PROTOCOL_VERSIONS_PROXY
    read -p "Superchain Config Proxy Address: " SUPERCHAIN_CONFIG_PROXY
    read -p "Superchain Proxy Admin Address: " SUPERCHAIN_PROXY_ADMIN
    read -p "L1 Proxy Admin Owner Address: " L1_PROXY_ADMIN_OWNER
    read -p "Challenger Address: " CHALLENGER
    
    OUTPUT_FILE="$OUTPUT_DIR/sepolia-implementations-$(date +%Y%m%d-%H%M%S).json"
else
    echo -e "${RED}Invalid choice. Exiting.${NC}"
    exit 1
fi

# Confirmation
echo ""
echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"
echo -e "${GREEN}Ready to deploy!${NC}"
echo ""
echo "  RPC URL: $L1_RPC_URL"
echo "  Output file: $OUTPUT_FILE"
if [ -n "$ETHERSCAN_API_KEY" ]; then
    echo "  Verification: ${GREEN}Enabled${NC} (Etherscan)"
else
    echo "  Verification: ${YELLOW}Disabled${NC} (no API key provided)"
fi
echo ""
echo -e "${YELLOW}⚠️  This will deploy contracts to Sepolia and consume ETH for gas!${NC}"
echo ""
read -p "Continue? [y/N]: " CONFIRM

if [[ ! "$CONFIRM" =~ ^[Yy]$ ]]; then
    echo -e "${RED}Deployment cancelled.${NC}"
    exit 0
fi

# Build the command
echo ""
echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"
echo -e "${GREEN}🚀 Starting deployment...${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"
echo ""

CMD=("$BINARY")

if [ "$DEPLOY_TYPE" == "1" ]; then
    CMD+=(
        "bootstrap" "superchain"
        "--l1-rpc-url" "$L1_RPC_URL"
        "--private-key" "$PRIVATE_KEY"
        "--outfile" "$OUTPUT_FILE"
        "--superchain-proxy-admin-owner" "$PROXY_ADMIN_OWNER"
        "--protocol-versions-owner" "$PROTOCOL_VERSIONS_OWNER"
        "--guardian" "$GUARDIAN"
    )
else
    CMD+=(
        "bootstrap" "implementations"
        "--l1-rpc-url" "$L1_RPC_URL"
        "--private-key" "$PRIVATE_KEY"
        "--outfile" "$OUTPUT_FILE"
        "--protocol-versions-proxy" "$PROTOCOL_VERSIONS_PROXY"
        "--superchain-config-proxy" "$SUPERCHAIN_CONFIG_PROXY"
        "--superchain-proxy-admin" "$SUPERCHAIN_PROXY_ADMIN"
        "--l1-proxy-admin-owner" "$L1_PROXY_ADMIN_OWNER"
        "--challenger" "$CHALLENGER"
        "--mips-version" "1"
    )
fi

# Add verification flags if API key provided
if [ -n "$ETHERSCAN_API_KEY" ]; then
    CMD+=(
        "--verify"
        "--verifier" "etherscan"
        "--verifier-api-key" "$ETHERSCAN_API_KEY"
    )
fi

# Execute the deployment
if "${CMD[@]}"; then
    echo ""
    echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"
    echo -e "${GREEN}✓ Deployment successful!${NC}"
    echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"
    echo ""
    echo -e "${GREEN}Output saved to:${NC} $OUTPUT_FILE"
    echo ""
    
    # Display deployed addresses
    if [ -f "$OUTPUT_FILE" ]; then
        echo -e "${BLUE}Deployed Addresses:${NC}"
        cat "$OUTPUT_FILE" | jq -r 'to_entries[] | "  \(.key): \(.value)"' 2>/dev/null || cat "$OUTPUT_FILE"
        echo ""
    fi
    
    # Verification status
    if [ -n "$ETHERSCAN_API_KEY" ]; then
        echo -e "${GREEN}✓ Contracts verified on Etherscan${NC}"
        echo "  View them at: https://sepolia.etherscan.io/"
    else
        echo -e "${YELLOW}ℹ  Run the verify command later to verify contracts:${NC}"
        echo ""
        echo "  $BINARY verify \\"
        echo "    --l1-rpc-url $L1_RPC_URL \\"
        echo "    --input-file $OUTPUT_FILE \\"
        echo "    --verifier-api-key YOUR_ETHERSCAN_API_KEY \\"
        echo "    --verifier etherscan \\"
        echo "    --artifacts-locator embedded"
    fi
    
    echo ""
    echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"
    
else
    echo ""
    echo -e "${RED}✗ Deployment failed!${NC}"
    echo ""
    echo "Check the error messages above for details."
    echo "Common issues:"
    echo "  - Insufficient Sepolia ETH in deployer account"
    echo "  - Invalid RPC URL"
    echo "  - Invalid private key format"
    echo "  - Network connectivity issues"
    echo ""
    exit 1
fi

