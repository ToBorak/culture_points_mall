# 文化积分商城 · 后端

> 双 binary：`cmd/server`（HTTP :8080） + `cmd/mcp`（JSON-RPC over SSE :8090）

## 快速启动

```bash
make up                              # 启动 MySQL + Redis
make migrate                         # 建表
go run ./cmd/migrate -action=seed    # 灌入 50 员工 / 3 部门 / 24 徽章 / 5 商品 / 8 奖品 + 演示积分

# 可选：配置 LLM API Key（HR-Agent 需要）
export ANTHROPIC_API_KEY="sk-ant-..."

make run                             # HTTP :8080
go run ./cmd/mcp                     # MCP :8090（另一终端）
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

详见 [`docs/superpowers/specs/2026-05-22-文化积分商城-动漫风钉钉应用-design.md`](docs/superpowers/specs/2026-05-22-文化积分商城-动漫风钉钉应用-design.md)
