# 文化积分商城 · AI 智能运营平台（后端）

> Go + Gin + GORM + MySQL + Redis · 模块化单体 + 双 binary（HTTP + MCP）

## 快速开始

```bash
# 1. 启动 MySQL + Redis
make up

# 2. 跑迁移
make migrate

# 3. 启动 HTTP 服务（默认 :8080）
make run

# 4. 跑测试
make test       # 单元
make test-int   # 集成（自动起 test docker）
```

## 设计

详见 [`docs/superpowers/specs/2026-05-22-文化积分商城-动漫风钉钉应用-design.md`](docs/superpowers/specs/2026-05-22-文化积分商城-动漫风钉钉应用-design.md)

## 路线图

详见 [`docs/superpowers/plans/2026-05-22-总路线图.md`](docs/superpowers/plans/2026-05-22-总路线图.md)
