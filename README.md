# dokploy-dingtalk-webhook

Dokploy Custom Webhook → 钉钉机器人 的消息转发中间层。

将 Dokploy 的 Custom Webhook 通知转换为钉钉机器人富文本卡片消息，零外部依赖的 Go 单二进制。

## 支持的通知类型

根据 Dokploy 发送的 payload 自动识别通知类型，生成对应的钉钉卡片：

| 通知类型 | 卡片样式 | 触发条件 |
|---|---|---|
| 构建成功 | ✅ ActionCard（带「查看构建详情」按钮） | `type=build, status=success` |
| 构建失败 | ❌ ActionCard + 错误信息 | `type=build, status=error` |
| 数据库备份成功 | ✅ Markdown 卡片 | `databaseType` 非空, `status=success` |
| 数据库备份失败 | ❌ Markdown 卡片 + 错误信息 | `databaseType` 非空, `status=error` |
| 服务器告警 | ⚠️ Markdown 卡片 (CPU/内存阈值) | `alertType=server-threshold` |
| 其他通知 | 📋 通用 Markdown 卡片 | Docker 清理、重启、卷备份等 |

当 payload 包含 `buildLink` 时，自动使用 ActionCard 消息类型（底部带可点击按钮）。

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

## Dokploy 发送的 Payload 字段

Dokploy Custom Webhook 根据不同事件类型发送不同的 JSON 字段，本服务全部支持：

| 字段 | 类型 | 说明 |
|---|---|---|
| `title` | string | 通知标题 |
| `message` | string | 通知消息 |
| `timestamp` | string | ISO 8601 时间戳 |
| `date` | string | 格式化日期 |
| `status` | string | 状态: `success` / `error` / `alert` |
| `type` | string | 事件类型: `build` / `CPU` / `Memory` |
| `projectName` | string | 项目名称 |
| `applicationName` | string | 应用名称 |
| `applicationType` | string | 应用类型 (docker, git 等) |
| `environmentName` | string | 环境名称 |
| `buildLink` | string | 构建日志链接 |
| `domains` | string | 域名列表 |
| `errorMessage` | string | 错误信息 |
| `databaseType` | string | 数据库类型 |
| `databaseName` | string | 数据库名称 |
| `serverName` | string | 服务器名称 |
| `alertType` | string | 告警类型 |
| `currentValue` | number | 当前值 (服务器指标) |
| `threshold` | number | 告警阈值 |

## 本地开发

```bash
go run .
```

## 接口

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/webhook/{access_token}?secret={secret}` | 接收 Dokploy 通知并转发至钉钉（secret 可选） |
| GET | `/health` | 健康检查 |
