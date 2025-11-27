#!/bin/bash
# Run integration tests for the scooter rental service
# This script starts Docker services, runs tests, and cleans up

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

echo "📁 Project root: $PROJECT_ROOT"

# Navigate to project root
cd "$PROJECT_ROOT"

# Check if docker-compose is available
if ! command -v docker-compose &> /dev/null; then
    echo "❌ docker-compose is not installed"
    exit 1
fi

# Check if Python is available
if ! command -v python3 &> /dev/null; then
    echo "❌ python3 is not installed"
    exit 1
fi

# Create virtual environment if it doesn't exist
VENV_DIR="$PROJECT_ROOT/integration_tests/.venv"
if [ ! -d "$VENV_DIR" ]; then
    echo "📦 Creating virtual environment..."
    python3 -m venv "$VENV_DIR"
fi

# Activate virtual environment and install dependencies
echo "📦 Installing test dependencies..."
source "$VENV_DIR/bin/activate"
pip install -q -r "$PROJECT_ROOT/integration_tests/requirements.txt"

# Run tests
echo "🧪 Running integration tests..."
cd "$PROJECT_ROOT/integration_tests"

# Pass any arguments to pytest
pytest "${@:-}" 

echo "✅ Tests completed!"
