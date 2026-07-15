# HubProxy 增强版

<p align="center">
  <strong>自托管 · 多源镜像加速 · 多用户管理后台</strong>
</p>

<p align="center">
  基于 <a href="https://github.com/sky22333/hubproxy">sky22333/hubproxy</a> 二次开发的增强版<br/>
  在保留原版 Docker / GitHub / Hugging Face 加速能力的基础上，增加完整管理后台、多用户令牌体系、拉取统计与安全策略。
</p>

<p align="center">
  <a href="https://github.com/sky22333/hubproxy">原版仓库</a> ·
  <a href="https://github.com/LiuShen-Fork/hubproxy">本仓库</a> ·
  <a href="https://www.liushen.fun/">作者主页</a>
</p>

---

## 项目说明

| | 说明 |
|---|---|
| **上游** | [sky22333/hubproxy](https://github.com/sky22333/hubproxy) |
| **本分支** | [LiuShen-Fork/hubproxy](https://github.com/LiuShen-Fork/hubproxy) |
| **定位** | 原版能力 + 可运营的多用户后台增强 |

本项目**继承原版**的代理核心（Registry API v2、GitHub/HF 加速、离线打包、镜像搜索等），并面向**自用 / 小团队内部分发**场景，补齐账号体系、访问令牌、配额统计与站点可配置能力。

> 感谢原作者 [sky22333](https://github.com/sky22333) 的开源工作。若你只需要轻量单机加速、不需要多用户后台，可直接使用原版。

---

## 增强版新增能力

### 管理后台

- 管理员 / 普通用户双控制台（侧栏可切换）
- 数据大屏：拉取次数、流量、独立 IP、趋势与 Top 列表
- 拉取记录 / 镜像统计 / IP 分析（筛选、分页、限高表格）
- 功能开关：Docker Hub、GitHub、HF、搜索、离线包、公共镜像
- 安全限流：全局限流、IP 黑白名单、仓库访问控制
- 系统设置：站点名称、备案、公告、OAuth2、SMTP

### 多用户与访问令牌

- 默认管理员：`admin` / `admin12346`（**首次登录强制改密**）
- 默认关闭表单注册；可按需开启注册 / 邮箱验证码
- 每用户 **8 位访问令牌**（全局唯一，重置后旧令牌作废）
- 拉取路径：`docker pull 域名/令牌/镜像`
- 支持 `registry-mirrors`：`https://域名/令牌`
- 用户侧：我的概览、令牌与快捷命令、IP 白名单、账户资料
- 每用户 **每日拉取配额**（默认 30 次/天，0 点刷新，可在用户管理中调整）

### 拉取会话统计

- Manifest 只跟踪不计次
- 任意一层 Blob 成功 → 计 **1 次**完整拉取
- 再次 Manifest → 开启新一轮（连续两次 pull 同一镜像会计 2 次）
- 纯 Manifest 探测超时自动清理

### 站点与认证扩展

- 站点名称、标语、HTML 公告弹窗
- ICP / 公安备案号（空则不显示，链接自动生成）
- 通用 OAuth2.0 登录 / 注册 / 绑定
- SMTP 配置与测试发信

### 安全加固（摘要）

- 密码 bcrypt、会话 token 仅存哈希
- 登录 IP / 用户名双维度节流
- 默认密码未修改前限制业务 API
- 安全响应头、Cookie HttpOnly / SameSite
- 浏览器访问 `/令牌` 路径返回 404 JSON，降低索引泄露风险

---

## 原版核心能力（保留）

- Docker 镜像加速（Registry API v2，流式传输，Manifest / Token 缓存）
- 多 Registry：Docker Hub、ghcr.io、gcr.io、quay.io、registry.k8s.io、registry.gitlab.com
- GitHub 文件 / Release / Clone / API 加速
- Hugging Face 模型与 LFS 加速
- 离线镜像 tar 打包下载
- 镜像搜索（Web + API）
- 统一 `config.toml` + 环境变量

---

## 快速开始

### Docker 部署

```bash
docker run -d \
  --name hubproxy \
  -p 5000:5000 \
  -v ./data:/app/data \
  -v ./src/config.toml:/app/config.toml:ro \
  --restart always \
  # 请替换为你构建/发布的镜像名
  your-registry/hubproxy:latest
```

验证：

```bash
curl http://127.0.0.1:5000/ready
```

### 本地开发

```bash
# 后端
cd src
export CONFIG_PATH=./config.toml   # Windows: $env:CONFIG_PATH="./config.toml"
go run .

# 前端（可选，热更新）
cd web
npm ci
npm run dev
```

- 前台：`http://127.0.0.1:5000/`
- 管理后台：`http://127.0.0.1:5000/admin`
- 默认账号：`admin` / `admin12346`（登录后请立即改密）
- SQLite 数据：`src/data/hubproxy.db`（可用 `databasePath` 配置）

### 生产构建

```bash
cd web && npm ci && npm run build   # 产物写入 src/dist
cd ../src && go build -o hubproxy .
```

---

## 使用示例

将 `yourdomain.com` 换成你的域名；将 `TOKEN` 换成用户控制台中的 8 位令牌。

### Docker 拉取（推荐显式路径）

```bash
# 官方镜像
docker pull yourdomain.com/TOKEN/nginx:latest

# 用户镜像
docker pull yourdomain.com/TOKEN/library/nginx:latest

# GHCR
docker pull yourdomain.com/TOKEN/ghcr.io/owner/app:tag

# Kubernetes
docker pull yourdomain.com/TOKEN/registry.k8s.io/pause:3.9
```

### registry-mirrors（带令牌路径）

```json
{
  "registry-mirrors": ["https://yourdomain.com/TOKEN"]
}
```

之后可直接：

```bash
docker pull nginx:latest
```

> 请勿在浏览器中打开 `/TOKEN` 路径；该路径仅用于 Docker 客户端，网页访问会返回 404。

### GitHub / Hugging Face

```bash
# Release
wget "https://yourdomain.com/https://github.com/owner/repo/releases/download/v1.0.0/app.tar.gz"

# Git clone
git clone https://yourdomain.com/https://github.com/owner/repo.git
```

---

## 配置说明

| 项 | 说明 |
|---|---|
| `src/config.toml` | 服务端口、上游 Registry、缓存等基础配置 |
| `server.databasePath` | SQLite 路径（用户、会话、统计、动态设置） |
| 管理后台 → 系统设置 | 站点信息、注册策略、OAuth2、SMTP、公告 |
| 管理后台 → 功能开关 | 加速路径与各 Registry 开关 |
| 管理后台 → 安全限流 | 全局限流、黑白名单、拉取会话规则 |

动态配置写入 SQLite，多数项**热更新**，无需重启。

---

## 目录结构（简要）

```
hubproxy/
├── src/                 # Go 后端（含 embed 前端 dist）
│   ├── config.toml
│   ├── db/              # SQLite：用户、令牌、统计、设置
│   ├── handlers/        # 代理、管理 API、OAuth、邮件
│   └── dist/            # 前端构建产物
├── web/                 # Vue 3 + Vite 前台与管理控制台
├── docs/                # 原版文档站点（可参考）
└── docker-compose.yml
```

---

## 与原版的关系

```
sky22333/hubproxy（原版）
        │
        │  代理核心、配置模型、部署方式
        ▼
LiuShen-Fork/hubproxy（本增强版）
        │
        ├── 多用户 / 访问令牌 / 配额
        ├── 管理后台与数据统计
        ├── 站点可配置 / OAuth2 / SMTP
        └── 安全与会话策略增强
```

上游更新时，请优先关注代理核心与配置兼容；后台相关模块为本仓库独立扩展。

---

## 免责声明

- 本程序仅供学习与自用交流，请勿用于非法用途  
- 使用须遵守当地法律法规及上游服务条款  
- 作者不对使用者的任何行为承担责任  
- 公开部署请自行做好 HTTPS、防火墙与账号安全  

---

## 致谢

- 原项目：[sky22333/hubproxy](https://github.com/sky22333/hubproxy)  
- 本增强版维护：[LiuShen-Fork/hubproxy](https://github.com/LiuShen-Fork/hubproxy)  
- 作者主页：[liushen.fun](https://www.liushen.fun/)  

**如果这个项目对你有帮助，欢迎 Star 原版与本仓库。**
