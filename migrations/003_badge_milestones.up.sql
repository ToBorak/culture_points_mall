-- 勋章墙重设计：里程碑勋章需要"获取条件"文案
ALTER TABLE badges ADD COLUMN description VARCHAR(255) NOT NULL DEFAULT '' AFTER name;
