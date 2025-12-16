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

### 4. 服务器部署

```bash
# SSH 到 Node2
sshpass -p 'f3t7uCBeTCizT12' ssh -p 22222 root@152.53.240.159

# 拉取新镜像并重启（根据实际部署方式调整）
docker pull yinghua001/new-api:latest
kubectl rollout restart deployment/new-api -n one-api
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
# 示例格式
# - relay/channel/xxx.go  # 新增渠道
# - web/src/xxx.tsx       # UI 修改
```

## 联系 AI 续接上下文

下次找 AI 时，提供此文档路径即可快速续接：
```
@/Users/yinghua/Documents/fly/new-api/DEV_GUIDE.md
```
