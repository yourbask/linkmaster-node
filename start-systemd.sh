#!/bin/bash

# ============================================
# LinkMaster 节点端 systemd 启动脚本
# 用于 systemd 服务，直接运行二进制文件
# ============================================

set -e

# 脚本目录
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# 配置
BINARY_NAME="agent"
BACKEND_URL="${BACKEND_URL:-http://localhost:8080}"

# 拉取最新源码并编译
update_and_build() {
    # 检查是否在 Git 仓库中
    if [ ! -d ".git" ]; then
        return 0
    fi
    
    # 配置 Git safe.directory，解决所有权问题
    CURRENT_DIR=$(pwd)
    git config --global --add safe.directory "$CURRENT_DIR" 2>/dev/null || true
    
    # 拉取最新代码
    if git pull 2>&1 > /dev/null; then
        echo "代码更新完成"
    fi
    
    # 检查 Go 环境
    if ! command -v go > /dev/null 2>&1; then
        echo "错误: 未找到 Go 环境，无法编译" >&2
        exit 1
    fi
    
    # 更新依赖
    go mod download 2>&1 > /dev/null || true
    
    # 编译
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
        else
            echo "错误: 编译失败，未生成二进制文件" >&2
            exit 1
        fi
    else
        echo "错误: 编译失败" >&2
        exit 1
    fi
}

# 拉取最新源码并编译
update_and_build

# 设置环境变量
export BACKEND_URL="$BACKEND_URL"

# 直接运行二进制文件（systemd 会管理进程）
exec ./"$BINARY_NAME"

