-- 会话核心摘要：结束会话时由 AI 提炼，用作侧栏预览 + 跨会话记忆注入
ALTER TABLE agent_sessions ADD COLUMN summary VARCHAR(512) NOT NULL DEFAULT '' AFTER title;
