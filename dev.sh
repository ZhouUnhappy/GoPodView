#!/bin/bash

# GoPodView Frontend & Backend Startup Manager Script
# Starts frontend and backend processes in the background and exits immediately
# Usage:
#   ./dev.sh start [--project <path>] [--port <port>] [--log <dir>]    Start frontend and backend
#   ./dev.sh stop                                                      Stop frontend and backend (graceful shutdown)

set -e

COMMAND=""
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PID_FILE="$SCRIPT_DIR/.dev.pid"
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
    print_usage
}

print_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

start_dev() {
    # Validate LOG_DIR
    if [ -z "$LOG_DIR" ]; then
        print_error "LOG_DIR is not set"
        exit 1
    fi
    
    # Create log directory if not exists
    mkdir -p "$LOG_DIR"
    
    local backend_log="$LOG_DIR/backend.log"
    local frontend_log="$LOG_DIR/frontend.log"
    
    print_info "Starting GoPodView..."
    print_info "Backend:  http://localhost:$GO_PORT"
    print_info "Frontend: http://localhost:$VITE_PORT"
    print_info "Logs:     $LOG_DIR"
    
    # Clear old PID file
    : > "$PID_FILE"
    
    # Start backend
    print_info "Starting backend..."
    cd "$SCRIPT_DIR/backend"
	print_info "GO_PARAMS: $GO_PARAMS"
    go run main.go $GO_PARAMS > "$backend_log" 2>&1 &
    BACKEND_PID=$!
    echo "$BACKEND_PID" >> "$PID_FILE"
    print_info "Backend process ID: $BACKEND_PID"
    print_info "Backend log: $backend_log"
    
    # Give backend time to start
    sleep 1
    
    # Start frontend
    print_info "Starting frontend..."
    cd "$SCRIPT_DIR/frontend"
    CI=true VITE_PORT=$VITE_PORT VITE_GO_PORT=$GO_PORT npm run dev > "$frontend_log" 2>&1 &
    FRONTEND_PID=$!
    echo "$FRONTEND_PID" >> "$PID_FILE"
    print_info "Frontend process ID: $FRONTEND_PID"
    print_info "Frontend log: $frontend_log"
    
    print_info "Frontend and backend started in background. Use '$0 stop' to stop them."
}

stop_dev() {
    if [ ! -f "$PID_FILE" ]; then
        print_warn "Process file not found. All processes may already be stopped."
        return
    fi
    
    print_info "Stopping frontend and backend..."
    
    while IFS= read -r pid; do
        print_info "Stopping process $pid..."
        if ps -p "$pid" > /dev/null 2>&1; then
            print_info "Process $pid is running, sending SIGTERM..."
            kill "$pid" 2>/dev/null || true
            
            # Wait for process to end (max 5 seconds)
            local count=0
            while ps -p "$pid" > /dev/null 2>&1 && [ $count -lt 50 ]; do
                sleep 0.1
                count=$((count + 1))
            done
            
            if ps -p "$pid" > /dev/null 2>&1; then
                print_warn "Process $pid did not stop gracefully. Force killing..."
                kill -9 "$pid" 2>/dev/null || true
            fi
        fi
    done < "$PID_FILE"
    
    rm -f "$PID_FILE"
    print_info "All processes stopped"
}

restart_dev() {
    stop_dev
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
        exit 1
        ;;
esac
