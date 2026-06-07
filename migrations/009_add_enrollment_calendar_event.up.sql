-- 报名自动加入钉钉日程：记录每条报名所创建的钉钉日历事件ID，取消报名时据此删除。
ALTER TABLE activity_enrollments
  ADD COLUMN calendar_event_id VARCHAR(128) NOT NULL DEFAULT '';
