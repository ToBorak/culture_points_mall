-- 盲盒奖池关联「积分好物」：item_id 指向 mall_items（NULL = 无奖品/谢谢参与）
-- mall_items.charge_on_miss：盲盒「未中奖是否也扣分」开关（1=都扣，0=中奖才扣），仅 blindbox 使用
ALTER TABLE mall_blindbox_pool
  ADD COLUMN item_id BIGINT DEFAULT NULL,
  ADD KEY idx_pool_item (item_id);

ALTER TABLE mall_items
  ADD COLUMN charge_on_miss TINYINT(1) NOT NULL DEFAULT 1;
