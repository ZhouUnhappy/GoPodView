#!/bin/bash

# GoPodView Frontend & Backend Startup Manager Script
# Starts frontend and backend processes in the background and exits immediately
# Usage:
#   ./dev.sh start [--project <path>] [--port <port>] [--log <dir>]    Start frontend and backend
#   ./dev.sh stop                                                      Stop frontend and backend (graceful shutdown)

set -e

COMMAND=""
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LOG_DIR="$SCRIPT_DIR/.dev/logs"

VITE_PORT=5173
GO_PORT=8080
GO_PARAMS=""

# Color definitions
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

print_usage() {
    cat << EOF
Usage:
  $0 start [--project <path>] [--vite_port <vite_port>] [--go_port <go_port>] [--log <dir>]    Start frontend and backend
  $0 stop                                                      Stop frontend and backend (graceful shutdown)
  $0 restart                                                   Restart frontend and backend
EOF
}

print_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

print_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

kill_by_port() {
    local port=$1
    local name=$2
    local pids=$(lsof -Pi :"$port" -sTCP:LISTEN -t 2>/dev/null)
    if [ -n "$pids" ]; then
        print_info "Killing $name processes on port $port (PIDs: $pids)"
        echo "$pids" | xargs kill -9 2>/dev/null || true
        sleep 1
    fi
}

start_dev() {
    # Validate LOG_DIR
    if [ -z "$LOG_DIR" ]; then
        print_error "LOG_DIR is not set"
        exit 1
    fi
    
    # Check if ports are available
    check_port() {
        local port=$1
        local name=$2
        if lsof -Pi :"$port" -sTCP:LISTEN -t >/dev/null 2>&1; then
            print_error "$name port $port is already in use"
            echo "Run 'lsof -i :$port' to see which process is using it"
            exit 1
        fi
    }
    check_port "$VITE_PORT" "Frontend"
    check_port "$GO_PORT" "Backend"
    
    # Create log directory if not exists
    mkdir -p "$LOG_DIR"
    
    local backend_log="$LOG_DIR/backend.log"
    local frontend_log="$LOG_DIR/frontend.log"
    
    print_info "Starting GoPodView..."
    print_info "Backend:  http://localhost:$GO_PORT"
    print_info "Frontend: http://localhost:$VITE_PORT"
    print_info "Logs:     $LOG_DIR"
    
    # Start backend
    print_info "Starting backend..."
    cd "$SCRIPT_DIR/backend"
	print_info "GO_PARAMS: $GO_PARAMS"
    go run main.go $GO_PARAMS > "$backend_log" 2>&1 &
    print_info "Backend log: $backend_log"
    
    # Give backend time to start
    sleep 1
    
    # Start frontend
    print_info "Starting frontend..."
    cd "$SCRIPT_DIR/frontend"
    CI=true VITE_PORT=$VITE_PORT VITE_GO_PORT=$GO_PORT npm run dev > "$frontend_log" 2>&1 &
    print_info "Frontend log: $frontend_log"
    
    print_info "Frontend and backend started in background. Use '$0 stop' to stop them."
}

stop_dev() {
    print_info "Stopping frontend and backend..."
    
    # Kill by port for reliability (handles child processes)
    kill_by_port "$GO_PORT" "Backend"
    kill_by_port "$VITE_PORT" "Frontend"
    
    print_info "All processes stopped"
}

restart_dev() {
    stop_dev
    print_info "Waiting for ports to be released..."
    sleep 2
    start_dev
}

while [ $# -gt 0 ]; do
    case $1 in
        start)
            COMMAND="start"
            shift
            ;;
        stop)
            COMMAND="stop"
            shift
            ;;
        restart)
            COMMAND="restart"
            shift
            ;;
        --project)
            GO_PROJECT_PATH="$2"
            shift 2
            ;;
        --go_port)
            GO_PORT="$2"
            shift 2
            ;;
		--vite_port)
			VITE_PORT="$2"
			shift 2
			;;
        --log)
            LOG_DIR="$2"
            shift 2
            ;;
        *)
            print_error "Unknown argument: $1"
			print_usage
            exit 1
            ;;
    esac
done

if [ -n "$GO_PROJECT_PATH" ]; then
    GO_PARAMS="$GO_PARAMS --project $GO_PROJECT_PATH"
fi

GO_PARAMS="$GO_PARAMS --port $GO_PORT"
GO_PARAMS="$GO_PARAMS --frontend-port $VITE_PORT"

# Execute command
case "$COMMAND" in
    start)
        start_dev
        ;;
    stop)
        stop_dev
        ;;
    restart)
        restart_dev
        ;;
    "")
        print_usage
        ;;
    *)
        print_error "Unknown command: $COMMAND"
		print_usage
        exit 1
        ;;
esac
