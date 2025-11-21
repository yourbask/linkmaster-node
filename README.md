# LinkMaster Node

LinkMaster 节点服务，用于执行网络测试任务。

## 功能

- HTTP GET/POST 测试
- Ping 测试
- DNS 查询
- Traceroute 路由追踪
- Socket 连接测试
- TCPing 端口延迟测试
- FindPing IP段批量ping检测
- 持续 Ping/TCPing 测试
- 心跳上报

## 安装

### 方式一：一键安装（推荐）

从 GitHub 一键安装，只需指定后端服务器地址。安装脚本会自动克隆源码并编译安装：

```bash
# 基本用法（替换为实际的 GitHub 仓库地址）
curl -fsSL https://raw.githubusercontent.com/yourbask/linkmaster-node/main/install.sh | bash -s -- http://your-backend-server:8080

# 示例
# 节点和后端在同一服务器
curl -fsSL https://raw.githubusercontent.com/yourbask/linkmaster-node/main/install.sh | bash -s -- http://localhost:8080

# 节点和后端在不同服务器
curl -fsSL https://raw.githubusercontent.com/yourbask/linkmaster-node/main/install.sh | bash -s -- http://192.168.1.100:8080
```

**重要说明：**
- 节点端需要直接连接后端服务器（端口 8080），不是前端地址
- 前端通过 `/api` 路径代理到后端，但节点端不使用代理
- 如果后端在公网，使用公网 IP 或域名
- 如果后端在内网，使用内网 IP
- **本项目是独立的 GitHub 仓库**，与前后端项目分离

**安装说明：**
- 自动检测系统类型和架构
- 自动安装系统依赖（包括 Git 和 Go）
- 从 GitHub 克隆源码到 `/opt/linkmaster-node`
- 自动编译并安装二进制文件
- 自动创建 systemd 服务
- 自动启动服务并验证

**指定分支安装：**
```bash
GITHUB_BRANCH=develop curl -fsSL https://raw.githubusercontent.com/yourbask/linkmaster-node/main/install.sh | bash -s -- http://your-backend-server:8080
```

### 方式二：手动编译安装

```bash
# 克隆仓库
git clone https://github.com/yourbask/linkmaster-node.git
cd linkmaster-node

# 编译
go build -o agent ./cmd/agent

# 安装（需要root权限）
sudo cp agent /usr/local/bin/linkmaster-node
sudo chmod +x /usr/local/bin/linkmaster-node

# 或使用安装脚本
./install.sh http://your-backend-server:8080
```

### 方式三：手动运行

```bash
# 克隆仓库
git clone https://github.com/yourbask/linkmaster-node.git
cd linkmaster-node

# 运行（会自动拉取最新代码并编译）
BACKEND_URL=http://your-backend-server:8080 ./run.sh start
```

## 配置

### 环境变量

- `BACKEND_URL`: 后端服务地址（必需，默认: http://localhost:8080）
- `CONFIG_PATH`: 配置文件路径（可选，默认: config.yaml）

### 配置文件（可选）

创建 `config.yaml` 文件：

```yaml
server:
  port: 2200
backend:
  url: http://your-backend-server:8080
heartbeat:
  interval: 60
debug: false
```

## 运行脚本

使用 `run.sh` 脚本管理节点端。**每次启动时会自动拉取最新代码并重新编译**：

```bash
# 启动服务（会自动拉取最新代码并编译）
./run.sh start

# 停止服务
./run.sh stop

# 重启服务（会拉取最新代码并重新编译）
./run.sh restart

# 查看状态
./run.sh status

# 查看日志
./run.sh logs

# 查看完整日志
./run.sh logs-all

# 指定后端地址启动
BACKEND_URL=http://192.168.1.100:8080 ./run.sh start
```

**重要提示：**
- 启动脚本会自动执行 `git pull` 拉取最新代码
- 拉取代码后会自动执行 `go mod download` 更新依赖
- 然后自动编译生成新的二进制文件
- 如果 Git 拉取失败（如网络问题），会使用当前代码继续编译
- 如果编译失败，服务不会启动

## API

### POST /api/test

统一测试接口

```json
{
  "type": "ceGet|cePost|cePing|ceDns|ceTrace|ceSocket|ceTCPing|ceFindPing",
  "url": "测试目标",
  "params": {}
}
```

### POST /api/continuous/start

启动持续测试

```json
{
  "type": "ping|tcping",
  "target": "测试目标",
  "interval": 10,
  "max_duration": 60
}
```

### POST /api/continuous/stop

停止持续测试

```json
{
  "task_id": "任务ID"
}
```

### GET /api/continuous/status?task_id=xxx

查询任务状态

### GET /api/health

健康检查
# linkmaster-node
# linkmaster-node
