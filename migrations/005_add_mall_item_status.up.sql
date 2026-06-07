-- 商品上下架状态：on_shelf=在售（默认），off_shelf=已下架（保留记录、商城不显示、不可兑换，可一键上架恢复）
ALTER TABLE mall_items ADD COLUMN status VARCHAR(16) NOT NULL DEFAULT 'on_shelf' AFTER image_url;
