#!/bin/bash

# BK-BSCP Services Management Script
# Usage: ./build/start.sh [start|stop|restart|status|clean] [service_name]
#
# Features:
# - Automatic log cleanup: Each service's old log files are automatically cleaned before starting
# - Port Configuration: Specify custom ports for services (see SERVICE_PORTS array below)
# - PID management: Services are tracked with PID files for proper management
# - Signal handling: Script responds to SIGTERM/SIGINT for graceful shutdown
# - Single service operation: Specify service name as second parameter to operate on single service
#
# Examples:
# ./build/start.sh start                    # Start all services
# ./build/start.sh start bk-bscp-dataservice # Start only dataservice
# ./build/start.sh restart bk-bscp-ui       # Restart only UI service
# ./build/start.sh stop bk-bscp-authserver  # Stop only authserver
# ./build/start.sh status bk-bscp-apiserver # Check status of apiserver
#
# Port Configuration:
# To specify custom ports for services, uncomment and modify the SERVICE_PORTS array below.
# If no port is specified for a service, the --port parameter will not be used.
# Example: ["bk-bscp-dataservice"]="9090" will start dataservice with --port 9090
#
# Log Management:
# - Logs are stored in ./build/logs/ directory
# - Each service startup automatically cleans old logs with the same service name prefix
# - Use './build/start.sh clean' to manually clean all service logs




SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
CONFIG_FILE="./build/config/bk-bscp.yaml"
PID_DIR="./build/pids"
LOG_DIR="./build/logs"

# Create PID directory if not exists
mkdir -p "$PID_DIR"

export VAULT_ADDR=http://127.0.0.1:8200
# export VAULT_TOKEN=[vault token]
export BK_USER_FOR_TEST=admin
export BK_APP_CODE_FOR_TEST=bk-bscp

# Define services array
SERVICES=(
    "bk-bscp-dataservice"
    "bk-bscp-authserver"
    "bk-bscp-configserver"
    "bk-bscp-apiserver"
    "bk-bscp-ui"
    "bk-bscp-cacheservice"
    "bk-bscp-feedserver"
)

# Define service ports (optional, leave empty if not needed)
# Format: SERVICE_NAME:PORT
declare -A SERVICE_PORTS=(
    # ["bk-bscp-dataservice"]="9090"
    # ["bk-bscp-authserver"]="9091"
    # ["bk-bscp-configserver"]="9092"
    ["bk-bscp-apiserver"]="8081"
    # ["bk-bscp-ui"]="9094"
    # ["bk-bscp-cacheservice"]="9095"
    # ["bk-bscp-feedproxy"]="9096"
    # ["bk-bscp-feedserver"]="9097"
)



# Function to validate service name
validate_service() {
    local service=$1
    for valid_service in "${SERVICES[@]}"; do
        if [ "$service" = "$valid_service" ]; then
            return 0
        fi
    done
    return 1
}

# Function to clean service logs
clean_service_logs() {
    local service=$1
    # Create logs directory if not exists
    mkdir -p "$LOG_DIR"
    
    # Clean logs with service name prefix
    if [ -d "$LOG_DIR" ]; then
        echo "Cleaning old log files for $service..."
        # Remove files that start with service name
        find "$LOG_DIR" -name "${service}*" -type f -delete 2>/dev/null || true
        echo "Log cleanup completed for $service"
    fi
}


# Function to start a service
start_service() {
    local service=$1
    local pid_file="$PID_DIR/${service}.pid"
    local log_file="$LOG_DIR/${service}.log"
    local executable="./build/bk-bscp/${service}/${service}"
    
    # Clean old logs before starting
    clean_service_logs "$service"
    
    # Determine config file based on service
    local config_file="$CONFIG_FILE"

    if [ "$service" = "bk-bscp-ui" ]; then
        config_file="./build/config/bk-bscp-ui.yaml"
    fi

    if [ "$service" = "bk-bscp-cacheservice" ] || [ "$service" = "bk-bscp-feedserver" ]; then
        config_file="./build/config/bk-bscp-feed.yaml"
    fi  
    

    
    # Build command arguments
    local cmd_args=("-c" "$config_file")
    
    # Add port parameter if specified for this service
    if [ -n "${SERVICE_PORTS[$service]}" ]; then
        cmd_args+=("--port" "${SERVICE_PORTS[$service]}")
        echo "Starting $service with config $config_file and port ${SERVICE_PORTS[$service]}..."
    else
        echo "Starting $service with config $config_file..."
    fi
    
    if [ -f "$pid_file" ] && kill -0 "$(cat "$pid_file")" 2>/dev/null; then
        echo "Service $service is already running (PID: $(cat "$pid_file"))"
        return 0
    fi
    
    if [ ! -f "$executable" ]; then
        echo "Error: Executable $executable not found"
        return 1
    fi
    
    if [ ! -f "$config_file" ]; then
        echo "Error: Config file $config_file not found for service $service"
        return 1
    fi
    
    nohup "$executable" "${cmd_args[@]}" > "$log_file" 2>&1 &
    local pid=$!
    echo $pid > "$pid_file"
    
    # Wait a moment and check if process is still running
    sleep 2
    if kill -0 $pid 2>/dev/null; then
        echo "Service $service started successfully (PID: $pid)"
        return 0
    else
        echo "Failed to start $service"
        rm -f "$pid_file"
        return 1
    fi
}



# Function to stop a service
stop_service() {
    local service=$1
    local pid_file="$PID_DIR/${service}.pid"
    
    if [ ! -f "$pid_file" ]; then
        echo "Service $service is not running (no PID file)"
        return 0
    fi
    
    local pid=$(cat "$pid_file")
    if ! kill -0 "$pid" 2>/dev/null; then
        echo "Service $service is not running (stale PID file)"
        rm -f "$pid_file"
        return 0
    fi
    
    echo "Stopping $service (PID: $pid)..."
    kill "$pid"
    
    # Wait for graceful shutdown
    local count=0
    while kill -0 "$pid" 2>/dev/null && [ $count -lt 30 ]; do
        sleep 1
        count=$((count + 1))
    done
    
    if kill -0 "$pid" 2>/dev/null; then
        echo "Force killing $service..."
        kill -9 "$pid"
        sleep 1
    fi
    
    rm -f "$pid_file"
    echo "Service $service stopped"
}

# Function to check service status
check_status() {
    local service=$1
    local pid_file="$PID_DIR/${service}.pid"
    
    if [ ! -f "$pid_file" ]; then
        echo "$service: Not running"
        return 1
    fi
    
    local pid=$(cat "$pid_file")
    if kill -0 "$pid" 2>/dev/null; then
        echo "$service: Running (PID: $pid)"
        return 0
    else
        echo "$service: Not running (stale PID file)"
        rm -f "$pid_file"
        return 1
    fi
}

# Function to clean all service logs
clean_all_logs() {
    local logs_dir="logs"
    
    # Create logs directory if not exists
    mkdir -p "$logs_dir"
    
    echo "Cleaning all service log files..."
    for service in "${SERVICES[@]}"; do
        clean_service_logs "$service"
    done
    echo "All log cleanup completed"
}

# Function to start all services
start_all() {
    echo "Starting all BK-BSCP services..."
    local failed=0
    
    for service in "${SERVICES[@]}"; do
        if ! start_service "$service"; then
            failed=$((failed + 1))
        fi
    done
    
    if [ $failed -eq 0 ]; then
        echo "All services started successfully"
    else
        echo "$failed services failed to start"
        exit 1
    fi
}


# Function to stop all services
stop_all() {
    echo "Stopping all BK-BSCP services..."
    
    # Stop services in reverse order
    for ((i=${#SERVICES[@]}-1; i>=0; i--)); do
        stop_service "${SERVICES[i]}"
    done
    
    echo "All services stopped"
}

# Function to show status of all services
status_all() {
    echo "BK-BSCP Services Status:"
    echo "========================"
    
    local running=0
    local total=${#SERVICES[@]}
    
    for service in "${SERVICES[@]}"; do
        if check_status "$service"; then
            running=$((running + 1))
        fi
    done
    
    echo "========================"
    echo "Running: $running/$total services"
}

# Function to restart all services
restart_all() {
    echo "Restarting all BK-BSCP services..."
    stop_all
    sleep 3
    start_all
}

# Trap signals to stop all services when script is terminated
trap 'echo "Received signal, stopping all services..."; stop_all; exit 0' SIGTERM SIGINT

# Main script logic
ACTION="${1:-start}"
SERVICE_NAME="$2"

# If service name is provided, validate it
if [ -n "$SERVICE_NAME" ]; then
    if ! validate_service "$SERVICE_NAME"; then
        echo "Error: Invalid service name '$SERVICE_NAME'"
        echo "Valid services are: ${SERVICES[*]}"
        exit 1
    fi
fi

case "$ACTION" in
    start)
        if [ -n "$SERVICE_NAME" ]; then
            echo "Starting service: $SERVICE_NAME"
            start_service "$SERVICE_NAME"
        else
            start_all
        fi
        ;;
    stop)
        if [ -n "$SERVICE_NAME" ]; then
            echo "Stopping service: $SERVICE_NAME"
            stop_service "$SERVICE_NAME"
        else
            stop_all
        fi
        ;;
    restart)
        if [ -n "$SERVICE_NAME" ]; then
            echo "Restarting service: $SERVICE_NAME"
            stop_service "$SERVICE_NAME"
            sleep 2
            start_service "$SERVICE_NAME"
        else
            restart_all
        fi
        ;;
    status)
        if [ -n "$SERVICE_NAME" ]; then
            echo "Status for service: $SERVICE_NAME"
            echo "========================"
            check_status "$SERVICE_NAME"
        else
            status_all
        fi
        ;;
    clean)
        if [ -n "$SERVICE_NAME" ]; then
            echo "Cleaning logs for service: $SERVICE_NAME"
            clean_service_logs "$SERVICE_NAME"
        else
            clean_all_logs
        fi
        ;;
    *)
        echo "Usage: $0 {start|stop|restart|status|clean} [service_name]"
        echo ""
        echo "Commands:"
        echo "  start   - Start all services or specified service (default, auto-cleans logs)"
        echo "  stop    - Stop all services or specified service"
        echo "  restart - Restart all services or specified service (auto-cleans logs)"
        echo "  status  - Show status of all services or specified service"
        echo "  clean   - Clean all service log files or specified service logs"
        echo ""
        echo "Available services: ${SERVICES[*]}"
        echo ""
        echo "Examples:"
        echo "  $0 start                    # Start all services"
        echo "  $0 start bk-bscp-dataservice # Start only dataservice"
        echo "  $0 restart bk-bscp-ui       # Restart only UI service"
        echo "  $0 stop bk-bscp-authserver  # Stop only authserver"
        echo "  $0 status bk-bscp-apiserver # Check status of apiserver"
        exit 1
        ;;
esac


