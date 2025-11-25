-- MT4 Trading API Database Schema

-- 1. 用户表
CREATE TABLE `users` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键',
  `email` VARCHAR(255) NOT NULL COMMENT '邮箱',
  `nickname` VARCHAR(100) NOT NULL COMMENT '昵称',
  `password` VARCHAR(255) NOT NULL COMMENT '密码（加密存储）',
  `member_level` INT DEFAULT 0 COMMENT '会员等级（默认0，可空）',
  `member_expire_at` INT(10) DEFAULT NULL COMMENT '会员到期时间（10位时间戳，可空）',
  `invite_code` VARCHAR(50) NOT NULL COMMENT '邀请码',
  `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_email` (`email`),
  UNIQUE KEY `uk_invite_code` (`invite_code`),
  KEY `idx_member_level` (`member_level`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='用户表';

-- 2. MT4账户表
CREATE TABLE `mt4_accounts` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键',
  `user_id` BIGINT UNSIGNED NOT NULL COMMENT '用户id',
  `platform_id` BIGINT UNSIGNED NOT NULL COMMENT '平台id',
  `account` VARCHAR(100) NOT NULL COMMENT '账户',
  `password` VARCHAR(255) NOT NULL COMMENT '密码',
  `type` TINYINT DEFAULT 0 COMMENT '类型（0美元，1美分，可空，默认0）',
  `amount` DECIMAL(15,2) DEFAULT 0.00 COMMENT '金额（可空默认0.00）',
  `profit` DECIMAL(15,2) DEFAULT 0.00 COMMENT '收益（可空默认0.00）',
  `status` TINYINT NOT NULL DEFAULT 0 COMMENT '状态（默认0）',
  `remark` VARCHAR(500) DEFAULT NULL COMMENT '备注（可空字符）',
  `deleted_at` TIMESTAMP NULL DEFAULT NULL COMMENT '删除时间（软删除）',
  `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  KEY `idx_user_id` (`user_id`),
  KEY `idx_platform_id` (`platform_id`),
  KEY `idx_account` (`account`),
  KEY `idx_status` (`status`),
  KEY `idx_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='MT4账户表';

-- 3. EA表
CREATE TABLE `eas` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键',
  `name` VARCHAR(100) NOT NULL COMMENT 'EA名字',
  `type` VARCHAR(50) DEFAULT NULL COMMENT 'EA类型（字符串可空）',
  `profit` VARCHAR(50) DEFAULT NULL COMMENT '收益（字符串可空）',
  `description` TEXT DEFAULT NULL COMMENT '描述（字符串可空）',
  `status` TINYINT DEFAULT 0 COMMENT '状态（整数可空，默认0）',
  `sort` INT DEFAULT 0 COMMENT '排序（整数可空，默认0）',
  `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  KEY `idx_status` (`status`),
  KEY `idx_sort` (`sort`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='EA表';

-- 4. EA参数表
CREATE TABLE `ea_params` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键',
  `ea_id` BIGINT UNSIGNED NOT NULL COMMENT 'EA ID',
  `name` VARCHAR(100) NOT NULL COMMENT '参数名称',
  `label` VARCHAR(100) NOT NULL COMMENT '参数标签',
  `type` VARCHAR(50) NOT NULL COMMENT '类型（对应前端的表单类型比如number等）',
  `default_value` VARCHAR(255) DEFAULT NULL COMMENT '默认值（可空）',
  `min` DECIMAL(15,2) DEFAULT NULL COMMENT '最小值（可空）',
  `max` DECIMAL(15,2) DEFAULT NULL COMMENT '最大值（可空）',
  `required` TINYINT DEFAULT 0 COMMENT '是否必填（默认0，可空）',
  `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  KEY `idx_ea_id` (`ea_id`),
  CONSTRAINT `fk_ea_params_ea_id` FOREIGN KEY (`ea_id`) REFERENCES `eas` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='EA参数表';

-- 5. 订单表
CREATE TABLE `orders` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键',
  `user_id` BIGINT UNSIGNED NOT NULL COMMENT '用户ID',
  `ea_id` BIGINT UNSIGNED NOT NULL COMMENT 'EA ID',
  `mt4_account_id` BIGINT UNSIGNED NOT NULL COMMENT 'MT4账户ID',
  `symbol` VARCHAR(20) NOT NULL COMMENT '货币对',
  `status` TINYINT NOT NULL DEFAULT 0 COMMENT '状态（0运行中，1已停止等）',
  `params` TEXT DEFAULT NULL COMMENT 'EA参数（JSON字符串存储）',
  `deleted_at` TIMESTAMP NULL DEFAULT NULL COMMENT '删除时间（软删除）',
  `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  KEY `idx_user_id` (`user_id`),
  KEY `idx_ea_id` (`ea_id`),
  KEY `idx_mt4_account_id` (`mt4_account_id`),
  KEY `idx_symbol` (`symbol`),
  KEY `idx_status` (`status`),
  KEY `idx_deleted_at` (`deleted_at`),
  UNIQUE KEY `uk_user_ea_symbol_active` (`user_id`, `ea_id`, `symbol`, `status`),
  CONSTRAINT `fk_orders_user_id` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_orders_ea_id` FOREIGN KEY (`ea_id`) REFERENCES `eas` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_orders_mt4_account_id` FOREIGN KEY (`mt4_account_id`) REFERENCES `mt4_accounts` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='订单表（每个EA同一个品种用户只可以同时开一个）';

-- 6. 订单列表表（MT4订单详情）
CREATE TABLE `order_list` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键',
  `order_id` BIGINT UNSIGNED NOT NULL COMMENT '对应的所属order表的id',
  `ticket` BIGINT NOT NULL COMMENT 'MT4订单号',
  `open_time` TIMESTAMP NULL DEFAULT NULL COMMENT '开仓时间',
  `close_time` TIMESTAMP NULL DEFAULT NULL COMMENT '平仓时间',
  `symbol` VARCHAR(20) NOT NULL COMMENT '货币对',
  `type` TINYINT NOT NULL COMMENT '订单类型（0买入，1卖出等）',
  `lots` DECIMAL(10,2) NOT NULL COMMENT '手数',
  `open_price` DECIMAL(15,5) NOT NULL COMMENT '开仓价',
  `close_price` DECIMAL(15,5) DEFAULT NULL COMMENT '平仓价',
  `stop_loss` DECIMAL(15,5) DEFAULT NULL COMMENT '止损',
  `take_profit` DECIMAL(15,5) DEFAULT NULL COMMENT '止盈',
  `magic_number` BIGINT DEFAULT NULL COMMENT '魔术号',
  `swap` DECIMAL(15,2) DEFAULT NULL COMMENT '库存费',
  `commission` DECIMAL(15,2) DEFAULT NULL COMMENT '手续费',
  `profit` DECIMAL(15,2) DEFAULT NULL COMMENT '盈亏',
  `status` TINYINT NOT NULL DEFAULT 0 COMMENT '状态',
  `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  KEY `idx_order_id` (`order_id`),
  KEY `idx_ticket` (`ticket`),
  KEY `idx_symbol` (`symbol`),
  KEY `idx_status` (`status`),
  CONSTRAINT `fk_order_list_order_id` FOREIGN KEY (`order_id`) REFERENCES `orders` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='订单列表表（MT4订单详情）';

-- 7. 平台表（券商）
CREATE TABLE `platforms` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键',
  `parent_id` BIGINT UNSIGNED DEFAULT 0 COMMENT '上级id（可空，默认0）',
  `title` VARCHAR(100) NOT NULL COMMENT '券商名',
  `status` TINYINT DEFAULT 0 COMMENT '状态（可空默认0）',
  `server` INT DEFAULT NULL COMMENT '所属的服务器id（整数，可空）',
  `remark` VARCHAR(500) DEFAULT NULL COMMENT '备注（可空）',
  `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  KEY `idx_parent_id` (`parent_id`),
  KEY `idx_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='平台表（券商）';

-- 8. 货币对表
CREATE TABLE `symbols` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键',
  `title` VARCHAR(50) NOT NULL COMMENT '货币对名字',
  `sort` INT DEFAULT 0 COMMENT '排序（默认0可空）',
  `status` TINYINT DEFAULT 0 COMMENT '状态（默认0可空）',
  `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_title` (`title`),
  KEY `idx_sort` (`sort`),
  KEY `idx_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='货币对表';
