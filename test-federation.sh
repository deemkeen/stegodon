#!/bin/bash

# test-federation.sh
# Helper script to set up stegodon with ngrok for local federation testing

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

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

# Check if stegodon binary exists
if [ ! -f "./stegodon" ]; then
    print_error "stegodon binary not found. Please run 'go build' first."
    exit 1
fi

print_success "Found stegodon binary"

# Check if ngrok is installed
if ! command -v ngrok &> /dev/null; then
    print_error "ngrok is not installed."
    echo "Install with: brew install ngrok"
    exit 1
fi

print_success "Found ngrok at $(which ngrok)"

# Check if .env file exists for ngrok domain
if [ -f ".ngrok_domain" ]; then
    SAVED_DOMAIN=$(cat .ngrok_domain)
    print_info "Last used ngrok domain: $SAVED_DOMAIN"
fi

# Start ngrok in background if not already running
NGROK_RUNNING=$(pgrep -f "ngrok http" || echo "")

if [ -n "$NGROK_RUNNING" ]; then
    print_warning "ngrok is already running (PID: $NGROK_RUNNING)"
    print_info "Getting ngrok URL from API..."

    # Wait a moment for ngrok API to be ready
    sleep 2

    # Get the public URL from ngrok API
    NGROK_URL=$(curl -s http://localhost:4040/api/tunnels | grep -o '"public_url":"https://[^"]*' | grep -o 'https://.*' | head -1)

    if [ -z "$NGROK_URL" ]; then
        print_error "Could not get ngrok URL from API. Is ngrok running correctly?"
        echo "Try: pkill -f ngrok && ngrok http 9999"
        exit 1
    fi

    # Extract domain without https://
    NGROK_DOMAIN=$(echo $NGROK_URL | sed 's|https://||')
    print_success "Using ngrok domain: $NGROK_DOMAIN"
else
    print_info "Starting ngrok tunnel on port 9999..."
    ngrok http 9999 > /dev/null &
    NGROK_PID=$!

    print_info "Waiting for ngrok to start..."
    sleep 3

    # Get the public URL from ngrok API
    NGROK_URL=$(curl -s http://localhost:4040/api/tunnels | grep -o '"public_url":"https://[^"]*' | grep -o 'https://.*' | head -1)

    if [ -z "$NGROK_URL" ]; then
        print_error "Failed to get ngrok URL. Check if ngrok started correctly."
        echo "Visit http://localhost:4040 to see ngrok status"
        exit 1
    fi

    # Extract domain without https://
    NGROK_DOMAIN=$(echo $NGROK_URL | sed 's|https://||')
    print_success "ngrok started with domain: $NGROK_DOMAIN"

    # Save domain for reference
    echo $NGROK_DOMAIN > .ngrok_domain
fi

# Display connection info
echo ""
echo "======================================================================"
echo -e "${GREEN}ngrok Tunnel Active${NC}"
echo "======================================================================"
echo "Public URL:    https://$NGROK_DOMAIN"
echo "Local:         http://localhost:9999"
echo "Web Interface: http://localhost:4040"
echo "======================================================================"
echo ""

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
                echo "  - @$user@$NGROK_DOMAIN"
            done
        fi
    fi
else
    print_info "No existing database - will create on first run"
fi

echo ""
print_info "Starting stegodon with ActivityPub enabled..."
echo ""

# Start stegodon with proper configuration
export STEGODON_WITH_AP=true
export STEGODON_SSLDOMAIN=$NGROK_DOMAIN

# Display configuration
echo "======================================================================"
echo -e "${GREEN}stegodon Configuration${NC}"
echo "======================================================================"
echo "Host:          127.0.0.1"
echo "SSH Port:      23232"
echo "HTTP Port:     9999"
echo "SSL Domain:    $NGROK_DOMAIN"
echo "ActivityPub:   enabled"
echo "======================================================================"
echo ""

print_info "Connect via SSH with: ${GREEN}ssh localhost -p 23232${NC}"
echo ""
print_info "To stop stegodon: Press Ctrl+C"
print_info "To stop ngrok: ${YELLOW}pkill -f ngrok${NC}"
echo ""
print_success "Starting server..."
echo ""

# Run stegodon (this will block until Ctrl+C)
./stegodon

# Cleanup message (only shown if stegodon exits cleanly)
echo ""
print_info "stegodon stopped"
print_warning "ngrok is still running. Stop it with: ${YELLOW}pkill -f ngrok${NC}"
