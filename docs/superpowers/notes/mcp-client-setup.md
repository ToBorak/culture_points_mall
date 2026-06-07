# 接入文化官 MCP Server

启动后端 MCP：

```bash
make run               # 启动 HTTP 8080
go run ./cmd/mcp       # 启动 MCP 8090
```

在 Claude Desktop 的 `~/Library/Application Support/Claude/claude_desktop_config.json`：

```json
{
  "mcpServers": {
    "culture-points-mall": {
      "url": "http://localhost:8090/mcp/sse?session=cli-1",
      "transport": "sse",
      "headers": {
        "Authorization": "Bearer demo"
      }
    }
  }
}
```

> 注：当前 demo 接受任意非空 token。生产部署需把 `simpleAuth` 替换为正式 API Key 表查询。

调用示例（在 Claude Desktop 内）：

> 「列出当前所有 published 活动」→ 触发 tools/call list_activities。
