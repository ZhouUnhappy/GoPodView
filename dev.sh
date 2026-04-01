#!/bin/bash

# GoPodView Frontend & Backend Startup Manager Script
# Usage:
#   ./dev.sh start [project-path] [port]    # Start frontend and backend
#   ./dev.sh stop                            # Stop frontend and backend gracefully
#   ./dev.sh kill                            # Forcefully kill frontend and backend

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PID_FILE="$SCRIPT_DIR/.dev.pid"
PROJECT_PORT="${PORT:-8080}"
VITE_PORT=5173

# Color definitions
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

print_usage() {
    cat << EOF
Usage:
  $0 start [--project <path>] [--port <port>]    Start frontend and backend
  $0 stop                                        Stop frontend and backend (graceful shutdown)

Examples:
  $0 start
  $0 start --project /path/to/my-go-project
  $0 start --project /path/to/my-go-project --port 9000
  $0 stop
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
    local project_path="$1"
    local port="${2:-8080}"
    
    if [ -n "$project_path" ] && [ ! -d "$project_path" ]; then
        print_error "Project path does not exist: $project_path"
        exit 1
    fi
    
    print_info "Starting GoPodView..."
    print_info "Backend:  http://localhost:$port"
    print_info "Frontend: http://localhost:$VITE_PORT"
    if [ -n "$project_path" ]; then
        print_info "Project:  $project_path"
    else
        print_warn "No project path provided. Use the frontend to load a project."
    fi
    
    # Create PID file to store process IDs
    : > "$PID_FILE"
    
    # Start backend
    print_info "Starting backend..."
    cd "$SCRIPT_DIR/backend"
    if [ -n "$project_path" ]; then
        go run main.go --project "$project_path" --port "$port" &
    else
        go run main.go --port "$port" &
    fi
    BACKEND_PID=$!
    echo "$BACKEND_PID" >> "$PID_FILE"
    print_info "Backend process ID: $BACKEND_PID"
    
    # Give backend time to start
    sleep 1
    
    # Start frontend
    print_info "Starting frontend..."
    cd "$SCRIPT_DIR/frontend"
    npm run dev &
    FRONTEND_PID=$!
    echo "$FRONTEND_PID" >> "$PID_FILE"
    print_info "Frontend process ID: $FRONTEND_PID"
    
    print_info "Frontend and backend started. Press Ctrl+C to stop"
    
    # Set cleanup function
    cleanup() {
        print_warn "Stop signal received, shutting down processes..."
        stop_dev
    }
    
    trap cleanup SIGINT SIGTERM
    
    # Wait for all background processes
    wait
}

stop_dev() {
    if [ ! -f "$PID_FILE" ]; then
        print_warn "Process file not found. All processes may already be stopped."
        return
    fi
    
    print_info "Stopping frontend and backend..."
    
    while IFS= read -r pid; do
        if ps -p "$pid" > /dev/null 2>&1; then
            print_info "Stopping process $pid..."
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

# Main logic
COMMAND=""
PROJECT_PATH=""
PORT=""

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
        --project)
            PROJECT_PATH="$2"
            shift 2
            ;;
        --port)
            PORT="$2"
            shift 2
            ;;
        *)
            print_error "Unknown argument: $1"
            exit 1
            ;;
    esac
done

# Execute command
case "$COMMAND" in
    start)
        start_dev "$PROJECT_PATH" "$PORT"
        ;;
    stop)
        stop_dev
        ;;
    "")
        print_usage
        ;;
    *)
        print_error "Unknown command: $COMMAND"
        exit 1
        ;;
esac
