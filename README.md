# dokploy-dingtalk-webhook

Dokploy Custom Webhook → 钉钉机器人 的消息转发中间层。

将 Dokploy 的通知格式 `{title, message, timestamp}` 转换为钉钉机器人 Markdown 消息格式，零外部依赖的 Go 单二进制。

## 部署到 Dokploy

### 方式一：Docker 镜像（推荐）

推送代码到 GitHub 后，GitHub Action 会自动构建镜像并推送到 GHCR。在 Dokploy 中：

1. 创建新应用，选择 **Docker** 来源
2. 镜像填 `ghcr.io/<你的用户名>/dokploy-dingtalk-webhook:latest`
3. 设置环境变量（见下方）
4. 部署

### 方式二：Git 仓库源码构建

1. 创建新应用，选择 Git 仓库（指向本项目）
2. Dokploy 会使用 Dockerfile 自动构建
3. 设置环境变量（见下方）
4. 部署

### 环境变量

| 变量 | 必填 | 说明 |
|---|---|---|
| `PORT` | 否 | 监听端口，默认 `9119` |

### 配置 Dokploy 通知

部署完成后，将本服务的地址填入 Dokploy → Notifications → Custom Webhook。

`access_token` 直接作为 URL 路径参数传入，`secret`（加签密钥）作为可选 query 参数：

```
https://your-domain.com/webhook/YOUR_ACCESS_TOKEN
```

如使用加签安全方式：

```
https://your-domain.com/webhook/YOUR_ACCESS_TOKEN?secret=YOUR_SECRET
```

一个服务实例可同时为多个钉钉机器人转发通知，只需配置不同的 Webhook URL。

## 本地开发

```bash
go run .
```

## 接口

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/webhook/{access_token}?secret={secret}` | 接收 Dokploy 通知并转发至钉钉（secret 可选） |
| GET | `/health` | 健康检查 |
