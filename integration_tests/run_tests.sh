set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

echo "Project root: $PROJECT_ROOT"

cd "$PROJECT_ROOT"

if ! command -v docker compose &> /dev/null; then
    echo "docker compose is not installed"
    exit 1
fi

if ! command -v python3 &> /dev/null; then
    echo "python3 is not installed"
    exit 1
fi

VENV_DIR="$PROJECT_ROOT/integration_tests/.venv"
if [ ! -d "$VENV_DIR" ]; then
    echo "Creating virtual environment..."
    python3 -m venv "$VENV_DIR"
fi

echo "Installing test dependencies..."
source "$VENV_DIR/bin/activate"
pip install -q -r "$PROJECT_ROOT/integration_tests/requirements.txt"

echo "Running integration tests..."
cd "$PROJECT_ROOT/integration_tests"

pytest "${@:-}" 

echo "Tests completed!"
