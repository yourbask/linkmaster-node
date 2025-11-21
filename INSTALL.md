# LinkMaster 节点端安装指南

## 一句话安装

### 从 GitHub 安装（推荐）

```bash
curl -fsSL https://raw.githubusercontent.com/yourbask/linkmaster-node/main/install.sh | bash -s -- http://your-backend-server:8080
```

**替换说明：**
- `yourbask/linkmaster-node` - 独立的 node 项目 GitHub 仓库地址
- `http://your-backend-server:8080` - 替换为实际的后端服务器地址

**重要提示：**
- ⚠️ 节点端需要直接连接后端服务器（端口 8080），不是前端地址
- 前端通过 `/api` 路径代理到后端，但节点端不使用前端代理
- 如果节点和后端在同一服务器：使用 `http://localhost:8080`
- 如果节点和后端在不同服务器：使用 `http://backend-ip:8080` 或 `http://backend-domain:8080`
- **本项目是独立的 GitHub 仓库**，与前后端项目分离

**示例：**
```bash
# 如果后端服务器在 192.168.1.100:8080
curl -fsSL https://raw.githubusercontent.com/yourbask/linkmaster-node/main/install.sh | bash -s -- http://192.168.1.100:8080
```

### 指定分支安装

```bash
GITHUB_BRANCH=develop curl -fsSL https://raw.githubusercontent.com/yourbask/linkmaster-node/main/install.sh | bash -s -- http://your-backend-server:8080
```

## 安装步骤说明

安装脚本会自动完成以下步骤：

1. **检测系统** - 自动识别 Linux 发行版和 CPU 架构
2. **安装依赖** - 自动安装 Git、Go、ping、traceroute、dnsutils 等工具
3. **克隆源码** - 从 GitHub 克隆 node 项目源码到 `/opt/linkmaster-node`
4. **编译安装** - 自动编译源码并安装二进制文件
5. **创建服务** - 自动创建 systemd 服务文件（使用 run.sh 启动）
6. **启动服务** - 自动启动并设置开机自启
7. **验证安装** - 检查服务状态和健康检查

**注意：** 每次服务启动时会自动拉取最新代码并重新编译，确保使用最新版本。

## 安装后管理

### 查看服务状态

```bash
sudo systemctl status linkmaster-node
```

### 查看日志

```bash
# 实时日志
sudo journalctl -u linkmaster-node -f

# 最近50行日志
sudo journalctl -u linkmaster-node -n 50
```

### 重启服务

```bash
sudo systemctl restart linkmaster-node
```

### 停止服务

```bash
sudo systemctl stop linkmaster-node
```

### 禁用开机自启

```bash
sudo systemctl disable linkmaster-node
```

## 验证安装

### 检查进程

```bash
ps aux | grep linkmaster-node
```

### 检查端口

```bash
netstat -tlnp | grep 2200
# 或
ss -tlnp | grep 2200
```

### 健康检查

```bash
curl http://localhost:2200/api/health
```

应该返回：`{"status":"ok"}`

## 手动安装（不使用脚本）

如果无法使用一键安装脚本，可以手动安装：

### 1. 克隆源码并编译

```bash
# 克隆仓库
git clone https://github.com/yourbask/linkmaster-node.git /opt/linkmaster-node
cd /opt/linkmaster-node

# 安装 Go 环境（如果未安装）
# Ubuntu/Debian
sudo apt-get install -y golang-go

# CentOS/RHEL
sudo yum install -y golang

# 编译
go build -o agent ./cmd/agent

# 安装到系统目录
sudo cp agent /usr/local/bin/linkmaster-node
sudo chmod +x /usr/local/bin/linkmaster-node
```

### 2. 安装系统依赖

```bash
# Ubuntu/Debian
sudo apt-get update
sudo apt-get install -y ping traceroute dnsutils curl

# CentOS/RHEL
sudo yum install -y iputils traceroute bind-utils curl
```

### 3. 创建 systemd 服务

```bash
# 确保 run.sh 有执行权限
sudo chmod +x /opt/linkmaster-node/run.sh

sudo tee /etc/systemd/system/linkmaster-node.service > /dev/null <<EOF
[Unit]
Description=LinkMaster Node Service
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=/opt/linkmaster-node
ExecStart=/opt/linkmaster-node/run.sh start
Restart=always
RestartSec=5
Environment="BACKEND_URL=http://your-backend-server:8080"

[Install]
WantedBy=multi-user.target
EOF
```

**注意：** 使用 `run.sh` 启动的好处是每次启动会自动拉取最新代码并重新编译。

### 4. 启动服务

```bash
sudo systemctl daemon-reload
sudo systemctl enable linkmaster-node
sudo systemctl start linkmaster-node
```

**注意：** 确保 `BACKEND_URL` 环境变量指向后端服务器的实际地址和端口（默认 8080），不是前端地址。

## 防火墙配置

确保开放端口 2200：

```bash
# Ubuntu/Debian (ufw)
sudo ufw allow 2200/tcp

# CentOS/RHEL (firewalld)
sudo firewall-cmd --permanent --add-port=2200/tcp
sudo firewall-cmd --reload
```

## 常见问题

### 1. 克隆或编译失败

**问题：** 无法从 GitHub 克隆源码或编译失败

**解决：**
- 检查网络连接
- 确认 GitHub 仓库地址正确（独立的 node 项目仓库）
- 确认已安装 Git 和 Go 环境
- 手动克隆并编译：`git clone https://github.com/yourbask/linkmaster-node.git && cd linkmaster-node && go build -o agent ./cmd/agent`

### 2. 服务启动失败

**问题：** systemctl status 显示服务失败

**解决：**
```bash
# 查看详细日志
sudo journalctl -u linkmaster-node -n 100

# 检查后端地址是否正确
sudo systemctl cat linkmaster-node | grep BACKEND_URL

# 手动测试后端连接
curl http://your-backend-server:8080/api/public/nodes/online
```

### 3. 端口被占用

**问题：** 端口 2200 已被占用

**解决：**
```bash
# 查找占用进程
sudo lsof -i :2200

# 停止占用进程或修改配置
```

### 4. 无法连接后端

**问题：** 节点无法连接到后端服务器

**解决：**
- 检查后端地址是否正确（应该是 `http://backend-server:8080`，不是前端地址）
- 检查网络连通性：`ping your-backend-server`
- 检查端口是否开放：`telnet your-backend-server 8080` 或 `nc -zv your-backend-server 8080`
- 检查防火墙规则（确保后端服务器的 8080 端口开放）
- 检查后端服务是否运行：`curl http://your-backend-server:8080/api/public/nodes/online`
- 如果使用前端代理，节点端仍需要直接连接后端，不能使用前端地址

## 卸载

```bash
# 停止服务
sudo systemctl stop linkmaster-node
sudo systemctl disable linkmaster-node

# 删除服务文件
sudo rm /etc/systemd/system/linkmaster-node.service
sudo systemctl daemon-reload

# 删除二进制文件和源码目录
sudo rm /usr/local/bin/linkmaster-node
sudo rm -rf /opt/linkmaster-node
```

