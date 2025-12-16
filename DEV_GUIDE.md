# New-API 二开指南

## 项目信息

| 项目 | 值 |
|------|-----|
| 官方仓库 | https://github.com/Calcium-Ion/new-api |
| Fork 仓库 | https://github.com/ImogeneOctaviap794/new-api |
| 本地路径 | `/Users/yinghua/Documents/fly/new-api` |
| Docker 镜像 | `yinghua001/new-api:latest` |

## Git Remote 配置

```bash
origin    https://github.com/ImogeneOctaviap794/new-api.git  # 你的 fork（推送）
upstream  https://github.com/Calcium-Ion/new-api.git         # 官方仓库（同步）
```

## 日常开发工作流

### 1. 开发并提交

```bash
# 修改代码后
git add .
git commit -m "feat: 你的功能描述"
git push origin main
```

### 2. 同步官方更新

```bash
# 获取官方最新代码
git fetch upstream

# 合并到本地
git merge upstream/main

# 解决冲突（如有）后推送
git push origin main
```

### 3. 构建 Docker 镜像（自动）

**已配置 GitHub Actions 自动构建**：每次 `git push` 后自动构建并推送镜像。

- Workflow 文件：`.github/workflows/docker.yml`
- 触发条件：push 到 main 分支
- 输出镜像：`yinghua001/new-api:latest`

手动构建（如需）：
```bash
# 注意：Mac ARM64 构建的镜像在 AMD64 服务器上无法运行
# 建议使用 GitHub Actions 自动构建
docker build -t yinghua001/new-api:latest .
docker push yinghua001/new-api:latest
```

### 4. 本地开发环境

```bash
# 启动本地 new-api（连接本地 MySQL）
docker run -d --name new-api-dev \
  -p 3001:3000 \
  -e SQL_DSN="root:123456@tcp(host.docker.internal:3306)/new_api_dev" \
  -e SESSION_SECRET="dev-session-secret" \
  yinghua001/new-api:latest

# 查看日志
docker logs -f new-api-dev

# 停止并删除
docker stop new-api-dev && docker rm new-api-dev
```

本地 MySQL 信息：
- 容器名：mysql
- Root 密码：123456
- 开发数据库：new_api_dev
- 访问地址：http://localhost:3001

### 5. 服务器部署

```bash
# 更新 new-api-v3
kubectl set image deployment/new-api-v3 -n one-api new-api=yinghua001/new-api:latest
kubectl rollout status deployment/new-api-v3 -n one-api

# 更新 new-api-horizon（商业服务，谨慎）
kubectl set image deployment/new-api-horizon -n one-api new-api-horizon=yinghua001/new-api:latest
kubectl rollout status deployment/new-api-horizon -n one-api

# 或使用滚动重启（镜像不变时）
kubectl rollout restart deployment/new-api-v3 -n one-api
```

## 项目结构

```
new-api/
├── web/                 # 前端 (React + Vite)
│   ├── src/
│   └── package.json
├── controller/          # API 控制器
├── model/              # 数据模型
├── middleware/         # 中间件
├── relay/              # 中继转发逻辑
├── common/             # 公共工具
├── Dockerfile          # Docker 构建文件
├── docker-compose.yml  # Docker Compose 配置
└── DEV_GUIDE.md        # 本文档
```

## 技术栈

| 层 | 技术 |
|---|------|
| 前端 | React + Vite + Bun |
| 后端 | Go (Gin) |
| 数据库 | MySQL / PostgreSQL |
| 缓存 | Redis |

## 常见二开场景

### 添加新渠道类型

1. `relay/channel/` 下新建渠道文件
2. 注册到 `relay/channel/registry.go`
3. 前端 `web/src/` 添加对应配置界面

### 修改 UI

1. 前端代码在 `web/src/`
2. 修改后重新构建：`cd web && bun run build`

### 添加新 API 端点

1. `controller/` 添加控制器
2. `router/` 注册路由

## 相关服务

| 服务 | 端口 | 说明 |
|------|------|------|
| new-api-horizon | 30002 | 生产环境（商业服务） |
| one-api | 30300 | 主实例 |
| one-api-v2 | 30301 | V2 实例 |

## 二开注意事项

### 代码修改原则

1. **改动集中化** - 尽量把改动放在独立文件/模块，减少与官方代码冲突
2. **写注释标记** - 在修改处添加 `// CUSTOM:` 注释，方便定位
3. **避免改核心文件** - 如必须改，记录在下方"已修改文件"列表

### 同步官方更新

1. **定期同步** - 建议每周 `git fetch upstream && git merge upstream/main`
2. **合并冲突** - 你改过的文件如果官方也改了，需手动解决
3. **测试回归** - 合并后完整测试，确保功能正常

### 构建部署

1. **Mac 架构问题** - Mac ARM64 本地构建的镜像在服务器(AMD64)无法运行，用 GitHub Actions
2. **生产环境谨慎** - 参考 one-api 操作原则，优先滚动更新
3. **测试先行** - 本地测试通过再部署

### GitHub 配置

| 配置项 | 说明 |
|--------|------|
| `DOCKERHUB_USERNAME` | Docker Hub 用户名 (Repository Secret) |
| `DOCKERHUB_TOKEN` | Docker Hub Token，需 Read & Write 权限 |

### 已修改文件

记录你的二开改动，方便合并时参考：

```
# 2025-12-17: Extended Thinking 支持 (Snowflake Cortex 兼容)
- dto/openai_request.go          # 添加 ThinkingBlocks 结构体
- service/convert.go             # ClaudeToOpenAI: thinking->reasoning 转换
                                 # ResponseOpenAI2Claude: reasoning_content->thinking 转换 (含 signature)

# 2025-12-17: Claude 流式响应修复 (Claude Code 兼容)
- service/convert.go             # 修复 StreamResponseOpenAI2Claude 在 Done=true 时提前返回的问题
                                 # 确保流式响应包含完整的 stop 事件序列
```

### Extended Thinking 功能说明

**目的**: 让 OpenAI 格式渠道（如 snowflake-proxy）支持 Claude 风格的 thinking 参数。

**修改内容**:

| 文件 | 修改 | 说明 |
|------|------|------|
| `dto/openai_request.go` | 添加 `ThinkingBlocks` 字段 | 解析 OpenAI 响应中的 `thinking_blocks` |
| `service/convert.go` | `ClaudeToOpenAIRequest` | 将 Claude `thinking` 参数转为 OpenAI `reasoning` 参数 |
| `service/convert.go` | `ResponseOpenAI2Claude` | 将 OpenAI `thinking_blocks`/`reasoning_content` 转为 Claude `thinking` 内容块 (含 `signature`) |

**影响范围**:
- ✅ 只影响 **OpenAI 渠道处理 Claude 格式请求** 的场景
- ❌ 不影响正常的 Claude 渠道 (类型 14)

**使用方式**:
```bash
# Claude 格式请求 + thinking 参数
curl -X POST 'http://xxx/v1/messages' -d '{
  "model": "claude-xxx",
  "thinking": {"type": "enabled", "budget_tokens": 1024},
  ...
}'

# 或使用 -thinking 后缀 (snowflake-proxy 支持)
curl -X POST 'http://xxx/v1/chat/completions' -d '{
  "model": "claude-xxx-thinking",
  ...
}'
```

## 联系 AI 续接上下文

下次找 AI 时，提供此文档路径即可快速续接：
```
@/Users/yinghua/Documents/fly/new-api/DEV_GUIDE.md
```
