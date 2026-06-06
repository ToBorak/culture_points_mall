-- 清理演示种子数据（demo seed cleanup）
--
-- 背景：早期 seed 会生成 50 个演示用户（ding_user_id = u001..u050，name = 员工01..员工50），
-- 每人发放欢迎积分，导致排行榜被这些「假用户」占据，与真实钉钉用户对不上。
-- 现已用 seed.demo_data 开关（默认 false）阻止再次生成；本脚本清理 *已存在* 的演示数据。
--
-- ⚠️ 破坏性操作：会永久删除这 50 个演示用户及其积分/流水/订单。请先备份，确认后再执行。
-- 默认租户 tenant_id = 1；如有不同请改下面的 @tid。
-- 演示用户的判定与 seed.go 生成规则一致：ding_user_id 形如 uNNN 且 name 以「员工」开头。

SET @tid := 1;

-- 子表先删（本库无外键约束，但按依赖顺序更稳）
DELETE FROM user_dimension_scores
 WHERE user_id IN (
   SELECT id FROM users WHERE tenant_id = @tid AND ding_user_id REGEXP '^u[0-9]{3}$' AND name LIKE '员工%'
 );

DELETE FROM point_transactions
 WHERE user_id IN (
   SELECT id FROM users WHERE tenant_id = @tid AND ding_user_id REGEXP '^u[0-9]{3}$' AND name LIKE '员工%'
 );

DELETE FROM mall_orders
 WHERE user_id IN (
   SELECT id FROM users WHERE tenant_id = @tid AND ding_user_id REGEXP '^u[0-9]{3}$' AND name LIKE '员工%'
 );

DELETE FROM mall_blindbox_freeze
 WHERE user_id IN (
   SELECT id FROM users WHERE tenant_id = @tid AND ding_user_id REGEXP '^u[0-9]{3}$' AND name LIKE '员工%'
 );

-- 最后删演示用户本身
DELETE FROM users
 WHERE tenant_id = @tid AND ding_user_id REGEXP '^u[0-9]{3}$' AND name LIKE '员工%';
