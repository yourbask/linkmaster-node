#!/bin/bash

# ============================================
# LinkMaster 节点端运行脚本
# 用途：启动、停止、重启节点端服务
# ============================================

set -e

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 脚本目录
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# 配置
BINARY_NAME="agent"
LOG_FILE="node.log"
PID_FILE="node.pid"
BACKEND_URL="${BACKEND_URL:-http://localhost:8080}"

# 获取PID
get_pid() {
    if [ -f "$PID_FILE" ]; then
        PID=$(cat "$PID_FILE")
        if ps -p "$PID" > /dev/null 2>&1; then
            echo "$PID"
        else
            rm -f "$PID_FILE"
            echo ""
        fi
    else
        echo ""
    fi
}

# 拉取最新源码并编译
update_and_build() {
    echo -e "${BLUE}拉取最新源码...${NC}"
    
    # 检查是否在 Git 仓库中
    if [ ! -d ".git" ]; then
        echo -e "${YELLOW}警告: 当前目录不是 Git 仓库，跳过代码更新${NC}"
        return 0
    fi
    
    # 配置 Git safe.directory，解决所有权问题
    CURRENT_DIR=$(pwd)
    git config --global --add safe.directory "$CURRENT_DIR" 2>/dev/null || true
    
    # 拉取最新代码
    if git pull 2>&1; then
        echo -e "${GREEN}✓ 代码更新完成${NC}"
    else
        PULL_EXIT_CODE=$?
        echo -e "${YELLOW}警告: Git 拉取失败（退出码: $PULL_EXIT_CODE），将使用当前代码继续${NC}"
        echo -e "${YELLOW}可能原因: 网络问题、权限问题或本地有未提交的更改${NC}"
    fi
    
    # 检查 Go 环境
    if ! command -v go > /dev/null 2>&1; then
        echo -e "${RED}错误: 未找到 Go 环境，无法编译${NC}"
        exit 1
    fi
    
    # 更新依赖
    echo -e "${BLUE}更新 Go 依赖...${NC}"
    if ! go mod download 2>&1; then
        echo -e "${YELLOW}警告: 依赖更新失败，尝试继续编译${NC}"
    else
        echo -e "${GREEN}✓ 依赖更新完成${NC}"
    fi
    
    # 编译
    echo -e "${BLUE}编译二进制文件...${NC}"
    ARCH=$(uname -m)
    case $ARCH in
        x86_64)
            ARCH="amd64"
            ;;
        aarch64|arm64)
            ARCH="arm64"
            ;;
        *)
            ARCH="amd64"
            ;;
    esac
    
    if GOOS=linux GOARCH=${ARCH} CGO_ENABLED=0 go build -buildvcs=false -ldflags="-w -s" -o "$BINARY_NAME" ./cmd/agent 2>&1; then
        if [ -f "$BINARY_NAME" ] && [ -s "$BINARY_NAME" ]; then
            chmod +x "$BINARY_NAME"
            echo -e "${GREEN}✓ 编译成功${NC}"
        else
            echo -e "${RED}错误: 编译失败，未生成二进制文件${NC}"
            exit 1
        fi
    else
        echo -e "${RED}错误: 编译失败${NC}"
        exit 1
    fi
}

# 检查二进制文件
check_binary() {
    if [ ! -f "$BINARY_NAME" ]; then
        echo -e "${RED}错误: 找不到二进制文件 $BINARY_NAME${NC}"
        echo -e "${YELLOW}尝试编译...${NC}"
        update_and_build
        return
    fi
    
    if [ ! -x "$BINARY_NAME" ]; then
        chmod +x "$BINARY_NAME"
    fi
}

# 检查端口占用
check_port() {
    if command -v lsof > /dev/null 2>&1; then
        PORT_PID=$(lsof -ti :2200 2>/dev/null || echo "")
        if [ -n "$PORT_PID" ]; then
            # 检查是否是我们的进程
            if [ -f "$PID_FILE" ] && [ "$PORT_PID" = "$(cat "$PID_FILE")" ]; then
                return 0
            fi
            echo -e "${YELLOW}警告: 端口2200已被占用 (PID: $PORT_PID)${NC}"
            echo -e "${YELLOW}是否要停止该进程? (y/n)${NC}"
            read -r answer
            if [ "$answer" = "y" ] || [ "$answer" = "Y" ]; then
                kill "$PORT_PID" 2>/dev/null || true
                sleep 1
            else
                echo -e "${RED}取消启动${NC}"
                exit 1
            fi
        fi
    fi
}

# 启动服务
start() {
    PID=$(get_pid)
    if [ -n "$PID" ]; then
        echo -e "${YELLOW}节点端已在运行 (PID: $PID)${NC}"
        return 0
    fi

    # 拉取最新源码并编译
    update_and_build
    
    check_port

    echo -e "${BLUE}启动节点端服务...${NC}"
    echo -e "${BLUE}后端地址: $BACKEND_URL${NC}"
    
    # 设置环境变量
    export BACKEND_URL="$BACKEND_URL"
    
    # 后台运行
    nohup ./"$BINARY_NAME" > "$LOG_FILE" 2>&1 &
    NEW_PID=$!
    
    # 保存PID
    echo "$NEW_PID" > "$PID_FILE"
    
    # 等待启动
    sleep 2
    
    # 检查是否启动成功
    if ps -p "$NEW_PID" > /dev/null 2>&1; then
        # 再次检查健康状态
        sleep 1
        if curl -s http://localhost:2200/api/health > /dev/null 2>&1; then
            echo -e "${GREEN}✓ 节点端已启动 (PID: $NEW_PID)${NC}"
            echo -e "${BLUE}日志文件: $LOG_FILE${NC}"
            echo -e "${BLUE}查看日志: tail -f $LOG_FILE${NC}"
        else
            echo -e "${GREEN}✓ 节点端进程已启动 (PID: $NEW_PID)${NC}"
            echo -e "${YELLOW}⚠ 健康检查未通过，请稍后查看日志${NC}"
        fi
    else
        echo -e "${RED}✗ 节点端启动失败${NC}"
        echo -e "${YELLOW}请查看日志: cat $LOG_FILE${NC}"
        rm -f "$PID_FILE"
        exit 1
    fi
}

# 停止服务
stop() {
    PID=$(get_pid)
    
    # 如果没有PID文件，尝试通过端口查找
    if [ -z "$PID" ]; then
        if command -v lsof > /dev/null 2>&1; then
            PORT_PID=$(lsof -ti :2200 2>/dev/null || echo "")
            if [ -n "$PORT_PID" ]; then
                PID="$PORT_PID"
                echo -e "${YELLOW}通过端口找到进程 (PID: $PID)${NC}"
            fi
        fi
    fi
    
    if [ -z "$PID" ]; then
        echo -e "${YELLOW}节点端未运行${NC}"
        rm -f "$PID_FILE"
        return 0
    fi

    echo -e "${BLUE}停止节点端服务 (PID: $PID)...${NC}"
    
    # 发送TERM信号
    kill "$PID" 2>/dev/null || true
    
    # 等待进程结束
    for i in {1..10}; do
        if ! ps -p "$PID" > /dev/null 2>&1; then
            break
        fi
        sleep 1
    done
    
    # 如果还在运行，强制杀死
    if ps -p "$PID" > /dev/null 2>&1; then
        echo -e "${YELLOW}强制停止节点端...${NC}"
        kill -9 "$PID" 2>/dev/null || true
        sleep 1
    fi
    
    rm -f "$PID_FILE"
    echo -e "${GREEN}✓ 节点端已停止${NC}"
}

# 重启服务
restart() {
    echo -e "${BLUE}重启节点端服务...${NC}"
    stop
    sleep 1
    start
}

# 查看状态
status() {
    PID=$(get_pid)
    if [ -n "$PID" ]; then
        echo -e "${GREEN}节点端运行中 (PID: $PID)${NC}"
        
        # 检查健康状态
        if command -v curl > /dev/null 2>&1; then
            HEALTH=$(curl -s http://localhost:2200/api/health 2>/dev/null || echo "failed")
            if [ "$HEALTH" = '{"status":"ok"}' ]; then
                echo -e "${GREEN}✓ 健康检查: 正常${NC}"
            else
                echo -e "${YELLOW}⚠ 健康检查: 异常${NC}"
            fi
        fi
        
        # 显示进程信息
        ps -p "$PID" -o pid,ppid,cmd,%mem,%cpu,etime 2>/dev/null || true
    else
        echo -e "${RED}节点端未运行${NC}"
    fi
}

# 查看日志
logs() {
    if [ -f "$LOG_FILE" ]; then
        tail -f "$LOG_FILE"
    else
        echo -e "${YELLOW}日志文件不存在: $LOG_FILE${NC}"
    fi
}

# 查看完整日志
logs_all() {
    if [ -f "$LOG_FILE" ]; then
        cat "$LOG_FILE"
    else
        echo -e "${YELLOW}日志文件不存在: $LOG_FILE${NC}"
    fi
}

# 显示帮助
help() {
    echo "LinkMaster 节点端运行脚本"
    echo ""
    echo "使用方法:"
    echo "  $0 {start|stop|restart|status|logs|logs-all|help}"
    echo ""
    echo "命令说明:"
    echo "  start      - 启动节点端服务（会自动拉取最新代码并编译）"
    echo "  stop       - 停止节点端服务"
    echo "  restart    - 重启节点端服务"
    echo "  status     - 查看运行状态"
    echo "  logs       - 实时查看日志"
    echo "  logs-all   - 查看完整日志"
    echo "  help       - 显示帮助信息"
    echo ""
    echo "环境变量:"
    echo "  BACKEND_URL - 后端服务地址 (默认: http://localhost:8080)"
    echo ""
    echo "示例:"
    echo "  BACKEND_URL=http://192.168.1.100:8080 $0 start"
    echo "  $0 status"
    echo "  $0 logs"
}

# 主逻辑
case "${1:-help}" in
    start)
        start
        ;;
    stop)
        stop
        ;;
    restart)
        restart
        ;;
    status)
        status
        ;;
    logs)
        logs
        ;;
    logs-all)
        logs_all
        ;;
    help|--help|-h)
        help
        ;;
    *)
        echo -e "${RED}未知命令: $1${NC}"
        echo ""
        help
        exit 1
        ;;
esac

