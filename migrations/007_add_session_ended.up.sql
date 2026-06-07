-- 会话「已结束」标记：结束会话后从历史列表移除（归档），但其 summary 仍用于跨会话记忆。
ALTER TABLE agent_sessions ADD COLUMN ended TINYINT NOT NULL DEFAULT 0 AFTER summary;
