#!/bin/sh
# ==============================================================================
# lightMonitor System Metrics Reporter Script
# Description: Multi-platform (Linux/macOS) compatible shell script to collect
#              system metrics (CPU, Memory, Disk Usage %) and report them to the
#              lightMonitor passive receive API.
# Compatibility: Linux (CentOS/RHEL, Ubuntu/Debian, Alpine/BusyBox, etc.), macOS
#
# Submitted JSON Payload Example:
# {
#   "group": "prod_servers",
#   "name": "web-host-01",
#   "token": "your_upload_token_here",
#   "timestamp": 1718706354,
#   "interval": 60,
#   "data": {
#     "cpu_usage": 12.50,
#     "mem_usage": 45.32,
#     "disk_usage": 35.10
#   }
# }
# ==============================================================================

# Exit on command errors if needed, but we handle fallbacks manually
# set -e

# Default values
API_URL=""
GROUP=""
NAME="$(hostname 2>/dev/null || echo "linux-server")"
TOKEN=""
INTERVAL=60
DISK_PATH="/"
DAEMON=false
DEBUG=false

# Color definitions for terminal output (only used if stdout is a TTY)
if [ -t 1 ]; then
    COLOR_RED='\033[0;31m'
    COLOR_GREEN='\033[0;32m'
    COLOR_YELLOW='\033[0;33m'
    COLOR_BLUE='\033[0;34m'
    COLOR_RESET='\033[0m'
else
    COLOR_RED=''
    COLOR_GREEN=''
    COLOR_YELLOW=''
    COLOR_BLUE=''
    COLOR_RESET=''
fi

log_info() {
    echo "${COLOR_BLUE}[INFO]${COLOR_RESET} $(date '+%Y-%m-%d %H:%M:%S') - $1"
}

log_success() {
    echo "${COLOR_GREEN}[SUCCESS]${COLOR_RESET} $(date '+%Y-%m-%d %H:%M:%S') - $1"
}

log_warn() {
    echo "${COLOR_YELLOW}[WARN]${COLOR_RESET} $(date '+%Y-%m-%d %H:%M:%S') - $1" >&2
}

log_error() {
    echo "${COLOR_RED}[ERROR]${COLOR_RESET} $(date '+%Y-%m-%d %H:%M:%S') - $1" >&2
}

# Print usage details
show_help() {
    cat <<EOF
lightMonitor System Metrics Reporter

Usage: $0 -u <api_url> -g <group_code> [options]

Required Options:
  -u, --url <url>       lightMonitor passive receive API URL
                        (e.g., http://localhost:8573/api/v1/receive)
  -g, --group <group>   Group code defined in lightMonitor

Optional Options:
  -n, --name <name>     Monitor name for this host (default: hostname)
  -t, --token <token>   Upload security token/secret (if required by system settings)
  -i, --interval <sec>  Data upload interval in seconds (default: 60)
  -d, --disk <path>     Disk partition mount path to monitor (default: /)
  -D, --daemon          Run in background loop (daemon mode)
  -x, --debug           Print detailed debug logs and output JSON payload
  -h, --help            Show this help message

Examples:
  # One-time execution:
  $0 -u http://127.0.0.1:8573/api/v1/receive -g prod_servers -n web-host-01

  # Run as daemon in background:
  $0 -u http://127.0.0.1:8573/api/v1/receive -g prod_servers -i 30 -D &
EOF
}

# Parse command line options
while [ $# -gt 0 ]; do
    case "$1" in
        -u|--url)
            API_URL="$2"
            shift 2
            ;;
        -g|--group)
            GROUP="$2"
            shift 2
            ;;
        -n|--name)
            NAME="$2"
            shift 2
            ;;
        -t|--token)
            TOKEN="$2"
            shift 2
            ;;
        -i|--interval)
            INTERVAL="$2"
            shift 2
            ;;
        -d|--disk)
            DISK_PATH="$2"
            shift 2
            ;;
        -D|--daemon)
            DAEMON=true
            shift
            ;;
        -x|--debug)
            DEBUG=true
            shift
            ;;
        -h|--help)
            show_help
            exit 0
            ;;
        *)
            log_error "Unknown option: $1"
            echo "Use -h or --help for usage information." >&2
            exit 1
            ;;
    esac
done

# Validate required arguments
if [ -z "$API_URL" ]; then
    log_error "Missing required option: -u, --url"
    show_help
    exit 1
fi

if [ -z "$GROUP" ]; then
    log_error "Missing required option: -g, --group"
    show_help
    exit 1
fi

# Ensure interval is a valid positive integer
if ! echo "$INTERVAL" | grep -qE '^[0-9]+$'; then
    log_error "Interval must be a positive integer: $INTERVAL"
    exit 1
fi

# ------------------------------------------------------------------------------
# Metric Gathering Functions
# ------------------------------------------------------------------------------

get_cpu_usage() {
    if [ "$(uname)" = "Darwin" ]; then
        # macOS fallback: Read CPU usage from top (needs 2 samples to be accurate)
        idle=$(top -l 2 -n 0 2>/dev/null | grep "CPU usage" | tail -n 1 | awk '{print $7}' | tr -d '%')
        if [ -n "$idle" ]; then
            cpu_val=$(awk "BEGIN {printf \"%.2f\", 100 - $idle}" 2>/dev/null)
            echo "${cpu_val:-0}"
        else
            echo "0"
        fi
    else
        # Linux implementation via /proc/stat
        if [ -f /proc/stat ]; then
            user=0; nice=0; system=0; idle=0; iowait=0; irq=0; softirq=0; steal=0; guest=0; guest_nice=0
            read -r cpu user nice system idle iowait irq softirq steal guest guest_nice < /proc/stat
            prev_idle=$((idle + iowait))
            prev_total=$((user + nice + system + idle + iowait + irq + softirq + steal + guest + guest_nice))
            
            sleep 1
            
            read -r cpu user nice system idle iowait irq softirq steal guest guest_nice < /proc/stat
            idle=$((idle + iowait))
            total=$((user + nice + system + idle + iowait + irq + softirq + steal + guest + guest_nice))
            
            diff_idle=$((idle - prev_idle))
            diff_total=$((total - prev_total))
            
            if [ "$diff_total" -eq 0 ]; then
                echo "0"
            else
                cpu_val=$(awk "BEGIN {printf \"%.2f\", ($diff_total - $diff_idle) * 100 / $diff_total}" 2>/dev/null)
                if [ -z "$cpu_val" ]; then
                    cpu_val=$((100 * (diff_total - diff_idle) / diff_total))
                fi
                echo "$cpu_val"
            fi
        else
            echo "0"
        fi
    fi
}

get_mem_usage() {
    if [ "$(uname)" = "Darwin" ]; then
        # macOS fallback: Calculate via vm_stat & sysctl
        total_mem=$(sysctl -n hw.memsize 2>/dev/null)
        page_size=$(vm_stat | grep "page size" | awk '{print $8}' 2>/dev/null)
        free_pages=$(vm_stat | grep "Pages free" | awk '{print $3}' | tr -d '.' 2>/dev/null)
        inactive_pages=$(vm_stat | grep "Pages inactive" | awk '{print $3}' | tr -d '.' 2>/dev/null)
        
        if [ -n "$total_mem" ] && [ -n "$page_size" ] && [ -n "$free_pages" ]; then
            [ -z "$inactive_pages" ] && inactive_pages=0
            avail_mem=$(((free_pages + inactive_pages) * page_size))
            used_mem=$((total_mem - avail_mem))
            mem_val=$(awk "BEGIN {printf \"%.2f\", $used_mem * 100 / $total_mem}" 2>/dev/null)
            echo "${mem_val:-0}"
        else
            echo "0"
        fi
    else
        # Linux implementation via /proc/meminfo
        if [ -f /proc/meminfo ]; then
            mem_val=$(awk '
                /MemTotal:/ { total = $2 }
                /MemFree:/ { free = $2 }
                /Buffers:/ { buffers = $2 }
                /Cached:/ { cached = $2 }
                /MemAvailable:/ { avail = $2 }
                /SReclaimable:/ { reclaimable = $2 }
                END {
                    if (avail > 0) {
                        used = total - avail
                    } else {
                        used = total - free - buffers - cached - reclaimable
                    }
                    if (total > 0) {
                        printf "%.2f", (used * 100 / total)
                    } else {
                        print "0"
                    }
                }
            ' /proc/meminfo 2>/dev/null)
            echo "${mem_val:-0}"
        else
            echo "0"
        fi
    fi
}

get_disk_usage() {
    # POSIX compliant df options
    disk_val=$(df -Pk "$DISK_PATH" 2>/dev/null | awk 'NR==2 {print $5}' | tr -d '%')
    if [ -z "$disk_val" ]; then
        disk_val=$(df -Pk / 2>/dev/null | awk 'NR==2 {print $5}' | tr -d '%')
    fi
    echo "${disk_val:-0}"
}

# ------------------------------------------------------------------------------
# Send Report Logic
# ------------------------------------------------------------------------------

run_report() {
    if [ "$DEBUG" = "true" ]; then
        log_info "Collecting system metrics..."
    fi

    # Read metrics
    cpu_usage=$(get_cpu_usage)
    mem_usage=$(get_mem_usage)
    disk_usage=$(get_disk_usage)

    # Format checks to ensure valid JSON representation
    if ! echo "$cpu_usage" | grep -qE '^[0-9]+(\.[0-9]+)?$'; then
        cpu_usage="0"
    fi
    if ! echo "$mem_usage" | grep -qE '^[0-9]+(\.[0-9]+)?$'; then
        mem_usage="0"
    fi
    if ! echo "$disk_usage" | grep -qE '^[0-9]+(\.[0-9]+)?$'; then
        disk_usage="0"
    fi

    timestamp=$(date +%s)

    # Build JSON payload manually (avoid jq dependency for better compatibility)
    PAYLOAD=$(cat <<EOF
{
  "group": "$GROUP",
  "name": "$NAME",
  "token": "$TOKEN",
  "timestamp": $timestamp,
  "interval": $INTERVAL,
  "data": {
    "cpu_usage": $cpu_usage,
    "mem_usage": $mem_usage,
    "disk_usage": $disk_usage
  }
}
EOF
)

    if [ "$DEBUG" = "true" ]; then
        log_info "Payload prepared:"
        echo "$PAYLOAD"
        log_info "Sending to $API_URL..."
    fi

    # POST payload using curl or wget
    http_code=0
    response_body=""

    if command -v curl >/dev/null 2>&1; then
        # Send using curl
        response=$(curl -s -w "\n%{http_code}" -X POST \
            -H "Content-Type: application/json" \
            -d "$PAYLOAD" \
            "$API_URL" 2>/dev/null)
        
        # Parse output and status code
        http_code=$(echo "$response" | tail -n 1)
        response_body=$(echo "$response" | sed '$d')
    elif command -v wget >/dev/null 2>&1; then
        # Send using wget (supports busybox/Alpine)
        tmp_resp=$(mktemp)
        if wget -q --header="Content-Type: application/json" \
            --post-data="$PAYLOAD" \
            -O "$tmp_resp" "$API_URL" 2>/dev/null; then
            http_code=200
            response_body=$(cat "$tmp_resp")
        else
            http_code=500
            response_body="wget execution failed"
        fi
        rm -f "$tmp_resp"
    else
        log_error "Neither curl nor wget was found on this system."
        exit 1
    fi

    # Handle API response
    if [ "$http_code" -eq 200 ] || [ "$http_code" -eq 201 ]; then
        if [ "$DEBUG" = "true" ] || [ "$DAEMON" = "false" ]; then
            log_success "Metrics reported successfully! Server Response: $response_body"
        fi
        
        # Dynamically adjust interval if returned by server
        # Response format: {"code":0,"msg":"success","data":{"interval":60}}
        server_interval=$(echo "$response_body" | grep -oE '"interval":\s*[0-9]+' | grep -oE '[0-9]+')
        if [ -n "$server_interval" ] && [ "$server_interval" -ne "$INTERVAL" ]; then
            log_info "Server requested interval change: $INTERVAL -> $server_interval"
            INTERVAL=$server_interval
        fi
    else
        log_warn "Failed to report metrics. HTTP Status Code: $http_code. Response: $response_body"
    fi
}

# ------------------------------------------------------------------------------
# Execution Entry Point
# ------------------------------------------------------------------------------

if [ "$DAEMON" = "true" ]; then
    log_info "Starting lightMonitor Metrics Reporter Daemon mode (Interval: ${INTERVAL}s)..."
    log_info "Group: $GROUP, Item Name: $NAME"
    while true; do
        run_report
        sleep "$INTERVAL"
    done
else
    run_report
fi
