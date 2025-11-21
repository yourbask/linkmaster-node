#!/bin/bash

# ============================================
# LinkMaster 节点端一键安装脚本
# 使用方法: curl -fsSL https://raw.githubusercontent.com/yourbask/linkmaster-node/main/install.sh | bash -s -- <后端地址>
# 示例: curl -fsSL https://raw.githubusercontent.com/yourbask/linkmaster-node/main/install.sh | bash -s -- http://192.168.1.100:8080
# ============================================

set -e

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# 配置
# 尝试从脚本URL自动提取仓库信息（如果通过curl下载）
SCRIPT_URL="${SCRIPT_URL:-}"
if [ -z "$SCRIPT_URL" ] && [ -n "${BASH_SOURCE[0]}" ]; then
    # 如果脚本是通过 curl 下载的，尝试从环境变量获取
    SCRIPT_URL="${SCRIPT_URL:-}"
fi

# 默认配置（如果无法自动提取，使用这些默认值）
GITHUB_REPO="${GITHUB_REPO:-yourbask/linkmaster-node}"  # 默认仓库（独立的 node 项目）
GITHUB_BRANCH="${GITHUB_BRANCH:-main}"  # 默认分支
SOURCE_DIR="/opt/linkmaster-node"  # 源码目录
BINARY_NAME="linkmaster-node"
INSTALL_DIR="/usr/local/bin"
SERVICE_NAME="linkmaster-node"

# 获取后端地址参数
BACKEND_URL="${1:-}"
if [ -z "$BACKEND_URL" ]; then
    echo -e "${RED}错误: 请提供后端服务器地址${NC}"
    echo -e "${YELLOW}使用方法:${NC}"
    echo "  curl -fsSL https://raw.githubusercontent.com/${GITHUB_REPO}/${GITHUB_BRANCH}/install.sh | bash -s -- http://your-backend-server:8080"
    echo ""
    echo -e "${YELLOW}注意:${NC}"
    echo "  - 节点端需要直接连接后端服务器，不是前端地址"
    echo "  - 后端默认端口: 8080"
    echo "  - 如果节点和后端在同一服务器: http://localhost:8080"
    echo "  - 如果节点和后端在不同服务器: http://backend-ip:8080 或 http://backend-domain:8080"
    exit 1
fi

# 检测系统类型和架构
detect_system() {
    if [ -f /etc/os-release ]; then
        . /etc/os-release
        OS=$ID
        OS_VERSION=$VERSION_ID
    else
        echo -e "${RED}无法检测系统类型${NC}"
        exit 1
    fi

    ARCH=$(uname -m)
    case $ARCH in
        x86_64)
            ARCH="amd64"
            ;;
        aarch64|arm64)
            ARCH="arm64"
            ;;
        *)
            echo -e "${RED}不支持的架构: $ARCH${NC}"
            exit 1
            ;;
    esac

    echo -e "${BLUE}检测到系统: $OS $OS_VERSION ($ARCH)${NC}"
}

# 安装系统依赖
install_dependencies() {
    echo -e "${BLUE}安装系统依赖...${NC}"
    
    if [ "$OS" = "ubuntu" ] || [ "$OS" = "debian" ]; then
        sudo apt-get update -qq
        sudo apt-get install -y -qq curl wget ping traceroute dnsutils git > /dev/null 2>&1
    elif [ "$OS" = "centos" ] || [ "$OS" = "rhel" ] || [ "$OS" = "rocky" ] || [ "$OS" = "almalinux" ]; then
        sudo yum install -y -q curl wget iputils traceroute bind-utils git > /dev/null 2>&1
    else
        echo -e "${YELLOW}警告: 未知系统类型，跳过依赖安装${NC}"
    fi
    
    echo -e "${GREEN}✓ 依赖安装完成${NC}"
}

# 安装 Go 环境
install_go() {
    echo -e "${BLUE}安装 Go 环境...${NC}"
    
    if [ "$OS" = "ubuntu" ] || [ "$OS" = "debian" ]; then
        sudo apt-get update -qq
        sudo apt-get install -y -qq golang-go > /dev/null 2>&1
    elif [ "$OS" = "centos" ] || [ "$OS" = "rhel" ] || [ "$OS" = "rocky" ] || [ "$OS" = "almalinux" ]; then
        sudo yum install -y -q golang > /dev/null 2>&1
    else
        echo -e "${YELLOW}无法自动安装 Go，请手动安装: https://golang.org/dl/${NC}"
        show_build_alternatives
        exit 1
    fi
    
    if command -v go > /dev/null 2>&1; then
        GO_VERSION=$(go version 2>/dev/null | head -1)
        echo -e "${GREEN}✓ Go 安装完成: ${GO_VERSION}${NC}"
    else
        echo -e "${RED}Go 安装失败${NC}"
        show_build_alternatives
        exit 1
    fi
}

# 显示替代方案
show_build_alternatives() {
    echo ""
    echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}"
    echo -e "${YELLOW}  安装失败，请使用以下替代方案:${NC}"
    echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}"
    echo ""
    echo -e "${GREEN}手动编译安装:${NC}"
    echo "  git clone https://github.com/${GITHUB_REPO}.git ${SOURCE_DIR}"
    echo "  cd ${SOURCE_DIR}"
    echo "  go build -o agent ./cmd/agent"
    echo "  sudo cp agent /usr/local/bin/linkmaster-node"
    echo "  sudo chmod +x /usr/local/bin/linkmaster-node"
    echo ""
}

# 检查是否已安装
check_installed() {
    # 检查服务文件是否存在
    if [ -f "/etc/systemd/system/${SERVICE_NAME}.service" ]; then
        return 0
    fi
    # 检查二进制文件是否存在
    if [ -f "$INSTALL_DIR/$BINARY_NAME" ]; then
        return 0
    fi
    # 检查源码目录是否存在
    if [ -d "$SOURCE_DIR" ]; then
        return 0
    fi
    return 1
}

# 卸载已安装的服务
uninstall_service() {
    echo -e "${BLUE}检测到已安装的服务，开始卸载...${NC}"
    
    # 停止服务
    if systemctl is-active --quiet ${SERVICE_NAME} 2>/dev/null; then
        echo -e "${BLUE}停止服务...${NC}"
        sudo systemctl stop ${SERVICE_NAME} 2>/dev/null || true
        sleep 2
    fi
    
    # 禁用服务
    if systemctl is-enabled --quiet ${SERVICE_NAME} 2>/dev/null; then
        echo -e "${BLUE}禁用服务...${NC}"
        sudo systemctl disable ${SERVICE_NAME} 2>/dev/null || true
    fi
    
    # 删除 systemd 服务文件
    if [ -f "/etc/systemd/system/${SERVICE_NAME}.service" ]; then
        echo -e "${BLUE}删除服务文件...${NC}"
        sudo rm -f /etc/systemd/system/${SERVICE_NAME}.service
    fi
    
    # 删除可能的 override 配置目录（包含 Environment 等配置）
    if [ -d "/etc/systemd/system/${SERVICE_NAME}.service.d" ]; then
        echo -e "${BLUE}删除服务配置目录...${NC}"
        sudo rm -rf /etc/systemd/system/${SERVICE_NAME}.service.d
    fi
    
    # 重新加载 systemd daemon
    sudo systemctl daemon-reload
    
    # 删除二进制文件
    if [ -f "$INSTALL_DIR/$BINARY_NAME" ]; then
        echo -e "${BLUE}删除二进制文件...${NC}"
        sudo rm -f "$INSTALL_DIR/$BINARY_NAME"
    fi
    
    # 删除源码目录
    if [ -d "$SOURCE_DIR" ]; then
        echo -e "${BLUE}删除源码目录...${NC}"
        sudo rm -rf "$SOURCE_DIR"
    fi
    
    # 清理进程（如果还在运行）
    if pgrep -f "$BINARY_NAME" > /dev/null 2>&1; then
        echo -e "${BLUE}清理残留进程...${NC}"
        sudo pkill -f "$BINARY_NAME" 2>/dev/null || true
        sleep 1
    fi
    
    echo -e "${GREEN}✓ 卸载完成${NC}"
    echo ""
}

# 从源码编译安装
build_from_source() {
    echo -e "${BLUE}从源码编译安装节点端...${NC}"
    
    # 检查 Go 环境
    if ! command -v go > /dev/null 2>&1; then
        echo -e "${BLUE}未检测到 Go 环境，开始安装...${NC}"
        install_go
    fi
    
    # 检查 Go 版本
    GO_VERSION=$(go version 2>/dev/null | head -1 || echo "")
    if [ -z "$GO_VERSION" ]; then
        echo -e "${RED}无法获取 Go 版本信息${NC}"
        show_build_alternatives
        exit 1
    fi
    
    echo -e "${BLUE}检测到 Go 版本: ${GO_VERSION}${NC}"
    
    # 如果源码目录已存在，删除（卸载函数应该已经删除，这里作为保险）
    if [ -d "$SOURCE_DIR" ]; then
        echo -e "${YELLOW}清理旧的源码目录...${NC}"
        sudo rm -rf "$SOURCE_DIR"
    fi
    
    # 克隆仓库到源码目录
    echo -e "${BLUE}克隆仓库到 ${SOURCE_DIR}...${NC}"
    if ! sudo git clone --branch "${GITHUB_BRANCH}" "https://github.com/${GITHUB_REPO}.git" "$SOURCE_DIR" 2>&1; then
        echo -e "${RED}克隆仓库失败，请检查网络连接和仓库地址${NC}"
        echo -e "${YELLOW}仓库地址: https://github.com/${GITHUB_REPO}.git${NC}"
        show_build_alternatives
        exit 1
    fi
    
    # 设置目录权限
    sudo chown -R $USER:$USER "$SOURCE_DIR" 2>/dev/null || true
    
    cd "$SOURCE_DIR"
    
    # 下载依赖
    echo -e "${BLUE}下载 Go 依赖...${NC}"
    if ! go mod download 2>&1; then
        echo -e "${RED}下载依赖失败${NC}"
        show_build_alternatives
        exit 1
    fi
    
    # 编译
    echo -e "${BLUE}编译二进制文件...${NC}"
    BINARY_PATH="$SOURCE_DIR/agent"
    if GOOS=linux GOARCH=${ARCH} CGO_ENABLED=0 go build -ldflags="-w -s" -o "$BINARY_PATH" ./cmd/agent 2>&1; then
        if [ -f "$BINARY_PATH" ] && [ -s "$BINARY_PATH" ]; then
            echo -e "${GREEN}✓ 编译成功${NC}"
        else
            echo -e "${RED}编译失败：未生成二进制文件${NC}"
            show_build_alternatives
            exit 1
        fi
    else
        echo -e "${RED}编译失败${NC}"
        show_build_alternatives
        exit 1
    fi
    
    # 复制到安装目录（可选，保留在源码目录供 run.sh 使用）
    sudo mkdir -p "$INSTALL_DIR"
    sudo cp "$BINARY_PATH" "$INSTALL_DIR/$BINARY_NAME"
    sudo chmod +x "$INSTALL_DIR/$BINARY_NAME"
    
    echo -e "${GREEN}✓ 编译安装完成${NC}"
    echo -e "${BLUE}源码目录: ${SOURCE_DIR}${NC}"
    echo -e "${BLUE}二进制文件: ${INSTALL_DIR}/${BINARY_NAME}${NC}"
}

# 创建 systemd 服务
create_service() {
    echo -e "${BLUE}创建 systemd 服务...${NC}"
    
    # 确保 run.sh 有执行权限
    sudo chmod +x "$SOURCE_DIR/run.sh"
    
    sudo tee /etc/systemd/system/${SERVICE_NAME}.service > /dev/null <<EOF
[Unit]
Description=LinkMaster Node Service
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=$SOURCE_DIR
ExecStart=$SOURCE_DIR/run.sh start
Restart=always
RestartSec=5
Environment="BACKEND_URL=$BACKEND_URL"

[Install]
WantedBy=multi-user.target
EOF

    sudo systemctl daemon-reload
    echo -e "${GREEN}✓ 服务创建完成${NC}"
}

# 启动服务
start_service() {
    echo -e "${BLUE}启动服务...${NC}"
    
    sudo systemctl enable ${SERVICE_NAME} > /dev/null 2>&1
    sudo systemctl restart ${SERVICE_NAME}
    
    # 等待服务启动
    sleep 3
    
    # 检查服务状态
    if sudo systemctl is-active --quiet ${SERVICE_NAME}; then
        echo -e "${GREEN}✓ 服务启动成功${NC}"
    else
        echo -e "${RED}✗ 服务启动失败${NC}"
        echo -e "${YELLOW}查看日志: sudo journalctl -u ${SERVICE_NAME} -n 50${NC}"
        exit 1
    fi
}

# 验证安装
verify_installation() {
    echo -e "${BLUE}验证安装...${NC}"
    
    # 检查进程
    if pgrep -f "$BINARY_NAME" > /dev/null; then
        echo -e "${GREEN}✓ 进程运行中${NC}"
    else
        echo -e "${YELLOW}⚠ 进程未运行${NC}"
    fi
    
    # 检查端口
    if command -v netstat > /dev/null 2>&1; then
        if netstat -tlnp 2>/dev/null | grep -q ":2200"; then
            echo -e "${GREEN}✓ 端口 2200 已监听${NC}"
        fi
    elif command -v ss > /dev/null 2>&1; then
        if ss -tlnp 2>/dev/null | grep -q ":2200"; then
            echo -e "${GREEN}✓ 端口 2200 已监听${NC}"
        fi
    fi
    
    # 健康检查
    sleep 2
    if curl -sf http://localhost:2200/api/health > /dev/null; then
        echo -e "${GREEN}✓ 健康检查通过${NC}"
    else
        echo -e "${YELLOW}⚠ 健康检查未通过，请稍后重试${NC}"
    fi
}

# 主安装流程
main() {
    echo -e "${GREEN}========================================${NC}"
    echo -e "${GREEN}  LinkMaster 节点端安装程序${NC}"
    echo -e "${GREEN}========================================${NC}"
    echo ""
    
    detect_system
    
    # 检查是否已安装，如果已安装则先卸载
    if check_installed; then
        uninstall_service
    fi
    
    install_dependencies
    build_from_source
    create_service
    start_service
    verify_installation
    
    echo ""
    echo -e "${GREEN}========================================${NC}"
    echo -e "${GREEN}  安装完成！${NC}"
    echo -e "${GREEN}========================================${NC}"
    echo ""
    echo -e "${BLUE}服务管理命令:${NC}"
    echo "  查看状态: sudo systemctl status ${SERVICE_NAME}"
    echo "  查看日志: sudo journalctl -u ${SERVICE_NAME} -f"
    echo "  重启服务: sudo systemctl restart ${SERVICE_NAME}"
    echo "  停止服务: sudo systemctl stop ${SERVICE_NAME}"
    echo ""
    echo -e "${BLUE}后端地址: ${BACKEND_URL}${NC}"
    echo -e "${BLUE}节点端口: 2200${NC}"
    echo ""
    echo -e "${YELLOW}重要提示:${NC}"
    echo "  - 节点端直接连接后端服务器，不使用前端代理"
    echo "  - 确保后端地址可访问: curl ${BACKEND_URL}/api/public/nodes/online"
}

# 执行安装
main
