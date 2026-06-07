# 文化官 · 后端

> 双 binary：`cmd/server`（HTTP :18080） + `cmd/mcp`（JSON-RPC over SSE :8090）

## 🚀 一键启动（推荐）

适合从零拉代码、本地没装过任何依赖的同学。**支持 macOS 与 Linux**（Windows 用户请走 WSL2）。

```bash
git clone <THIS_REPO>.git culture_points_mall
cd culture_points_mall
./bootstrap.sh
```

脚本会自动完成：
1. 检测并安装 **Homebrew / Docker / Go / Node / pnpm**（macOS 走 brew，Linux 走 apt）
2. 启动 **MySQL 8.4 + Redis 7**（Docker 容器）
3. **重置数据库**：drop → migrate → seed 50 员工 + 24 徽章 + 商品 + 奖品池
4. 克隆/检测前端仓库（默认在 `../culture_points_mall_web`）并 `pnpm install`
5. 拉起 **后端 :18080 / MCP :8090 / H5 :5173 / Admin :5174**，浏览器自动打开

> **首次克隆前端**：若同级目录没有前端仓库，需指定 git 地址
> ```bash
> FRONTEND_GIT=git@github.com:YOUR_ORG/culture-points-mall-web.git ./bootstrap.sh
> ```
>
> **保留旧数据**：`./bootstrap.sh --no-reset`
>
> **不自动开浏览器**：`./bootstrap.sh --no-open`

启动完成后访问：

| 服务 | URL | 备注 |
|---|---|---|
| 员工 H5 | http://localhost:5173 | 自动钉钉模拟登录，新用户首登赠 100,000 积分 |
| 管理后台 | http://localhost:5174 | User ID 输入 `1` 即可登录 |
| 后端 API | http://localhost:18080/healthz | |
| MCP 服务 | http://localhost:8090/mcp/sse | Bearer token 任意非空字符串 |

如果要用 HR-Agent 自然语言聊天，编辑 `configs/config.yaml` 填入 LLM API Key
（`llm.provider` 可选 `claude` / `openai` / `deepseek` / `qwen`）。

## 常用命令

```bash
make help        # 列出全部命令
make up          # 重启后端 + 前端进程（容器已在跑）
make down        # 停止所有进程
make reset       # 重置数据库（drop + migrate + seed）
make logs        # 实时跟踪日志
make ps          # 查看容器状态
```

## 仅本地开发（已搭好环境）

```bash
make up                              # 启动 MySQL + Redis 容器
make migrate                         # 建表
make seed                            # 灌入演示数据
make dev                             # 前台运行后端 :18080
go run ./cmd/mcp                     # 另一终端：MCP :8090
```

## 演示路径

1. **员工端 H5** http://localhost:5173
   - 浏览器开发模式自动登录 → `/passport` 看 3D 雷达水晶柱 + 徽章墙 + 流水
   - `/leaderboard` 看 GSAP 漫画领奖台（总榜 / 维度榜 / 部门榜 切换）
   - `/signin?a={活动ID}&c={code}` 扫码签到（实际从 admin 大屏拿 code）
   - `/mall` 商城 → `/mall/blindbox/{id}` 抽奖（3D 转盘 + WIN/差一点）

2. **HR 后台 PC** http://localhost:5174
   - userId=1 开发登录 → `/chat` 输入自然语言体验 HR-Agent
   - `/values` 价值观维度只读列表
   - `/activities/:id/code` 二维码大屏（WebSocket 自动刷新）
   - `/dingtalk/mock-outbox` 钉钉模拟推送面板

3. **MCP** Claude Desktop 配置：
   - `docs/superpowers/notes/mcp-client-setup.md`

## 测试

```bash
make test          # 单元测试
make test-int      # 集成测试（dockertest 自动起 MySQL + miniredis）
```

## 模块结构

- `internal/modules/` — DDD 风格业务模块（values / points / activities / signin / mall / achievements / leaderboard / agent）
- `internal/platform/` — 平台层（dingtalk Mock 适配层 / llm Factory / storage / mcp 协议运行时）
- `internal/auth/` — JWT + 钉钉免登
- `internal/migrate/` — 迁移与 seed
- `internal/router/` — 路由组装

详见 [`docs/superpowers/specs/2026-05-22-文化官-动漫风钉钉应用-design.md`](docs/superpowers/specs/2026-05-22-文化官-动漫风钉钉应用-design.md)
