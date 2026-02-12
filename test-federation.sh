#!/bin/bash

# test-federation.sh
# Helper script to set up stegodon with bore for local federation testing
# Uses https://github.com/jkuri/bore for tunneling

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
BORE_ID="${BORE_ID:-stegodon}"
BORE_DOMAIN="${BORE_ID}.bore.digital"
LOCAL_PORT=9999

# Function to print colored output
print_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Cleanup function
cleanup() {
    echo ""
    print_info "Shutting down..."

    # Stop bore first
    if [ -n "$BORE_PID" ] && kill -0 "$BORE_PID" 2>/dev/null; then
        print_info "Stopping bore tunnel (PID: $BORE_PID)"
        kill "$BORE_PID" 2>/dev/null || true
    fi

    # Stop stegodon
    if [ -n "$STEGODON_PID" ] && kill -0 "$STEGODON_PID" 2>/dev/null; then
        print_info "Stopping stegodon (PID: $STEGODON_PID)"
        kill "$STEGODON_PID" 2>/dev/null || true
        wait "$STEGODON_PID" 2>/dev/null || true
    fi

    print_success "Cleanup complete"
}

trap cleanup EXIT INT TERM

# Check if stegodon binary exists
if [ ! -f "./stegodon" ]; then
    print_error "stegodon binary not found. Please run 'go build' first."
    exit 1
fi

print_success "Found stegodon binary"

# Check if bore is installed
if ! command -v bore &> /dev/null; then
    print_error "bore is not installed."
    echo "Install from: https://github.com/jkuri/bore"
    echo "  brew install jkuri/tap/bore"
    echo "  or download from GitHub releases"
    exit 1
fi

print_success "Found bore at $(which bore)"

# Check if database exists
if [ -f "database.db" ]; then
    print_info "Using existing database.db"

    # Count existing accounts
    ACCOUNT_COUNT=$(sqlite3 database.db "SELECT COUNT(*) FROM accounts;" 2>/dev/null || echo "0")
    if [ "$ACCOUNT_COUNT" != "0" ]; then
        print_info "Found $ACCOUNT_COUNT existing account(s)"

        # Show usernames
        USERNAMES=$(sqlite3 database.db "SELECT username FROM accounts;" 2>/dev/null || echo "")
        if [ -n "$USERNAMES" ]; then
            echo "Existing users:"
            echo "$USERNAMES" | while read -r user; do
                echo "  - @$user@${BORE_DOMAIN}"
            done
        fi
    fi
else
    print_info "No existing database - will create on first run"
fi

echo ""

# Set up stegodon environment
export STEGODON_WITH_AP=true
export STEGODON_SSLDOMAIN=${BORE_DOMAIN}
export STEGODON_WITH_PPROF=true
export STEGODON_SHOW_GLOBAL=true
export STEGODON_SHOW_TOS=true

# Display configuration
echo "======================================================================"
echo -e "${GREEN}stegodon Configuration${NC}"
echo "======================================================================"
echo "Host:          127.0.0.1"
echo "SSH Port:      23232"
echo "HTTP Port:     ${LOCAL_PORT}"
echo "SSL Domain:    ${BORE_DOMAIN}"
echo "ActivityPub:   enabled"
echo "======================================================================"
echo ""

# Step 1: Start stegodon in background
print_info "Starting stegodon..."
./stegodon &
STEGODON_PID=$!

# Wait for stegodon to start listening
print_info "Waiting for stegodon to start..."
sleep 2

# Check if stegodon is still running
if ! kill -0 "$STEGODON_PID" 2>/dev/null; then
    print_error "stegodon failed to start"
    exit 1
fi

print_success "stegodon started (PID: $STEGODON_PID)"

# Step 2: Start bore tunnel
print_info "Starting bore tunnel: localhost:${LOCAL_PORT} -> https://${BORE_DOMAIN}"
bore -lp ${LOCAL_PORT} -id ${BORE_ID} &
BORE_PID=$!

sleep 2

# Check if bore is still running
if ! kill -0 "$BORE_PID" 2>/dev/null; then
    print_error "bore failed to start. Check if the ID '${BORE_ID}' is available."
    exit 1
fi

print_success "bore tunnel established (PID: $BORE_PID)"

# Display connection info
echo ""
echo "======================================================================"
echo -e "${GREEN}Federation Testing Ready${NC}"
echo "======================================================================"
echo "Public URL:    https://${BORE_DOMAIN}"
echo "Local:         http://localhost:${LOCAL_PORT}"
echo "Tunnel ID:     ${BORE_ID}"
echo "======================================================================"
echo ""

print_info "Connect via SSH: ${GREEN}ssh localhost -p 23232${NC}"
print_info "Web UI:          ${GREEN}https://${BORE_DOMAIN}${NC}"
echo ""
print_info "To stop: Press Ctrl+C (stops both stegodon and bore)"
echo ""

# Custom bore ID usage
if [ "$BORE_ID" = "stegodon" ]; then
    print_info "Tip: Use a custom ID with: ${YELLOW}BORE_ID=myname ./test-federation.sh${NC}"
    echo ""
fi

print_success "Ready for federation testing!"
echo ""

# Wait for either process to exit
wait $STEGODON_PID $BORE_PID
