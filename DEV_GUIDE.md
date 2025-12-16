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

### 3. 构建 Docker 镜像

```bash
# 本地构建
docker build -t yinghua001/new-api:latest .

# 推送到 Docker Hub
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

## 注意事项

1. **生产环境操作需谨慎** - 参考 one-api 操作原则
2. **测试先行** - 本地测试通过再部署
3. **保持同步** - 定期 `git fetch upstream` 获取官方更新
4. **记录变更** - 重要修改写入此文档

## 联系 AI 续接上下文

下次找 AI 时，提供此文档路径即可快速续接：
```
@/Users/yinghua/Documents/fly/new-api/DEV_GUIDE.md
```
