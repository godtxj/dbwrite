package models

import (
	"time"
)

// User 用户表
type User struct {
	ID              int64      `json:"id" db:"id"`                               // 主键
	Email           string     `json:"email" db:"email"`                         // 邮箱
	Nickname        string     `json:"nickname" db:"nickname"`                   // 昵称
	Password        string     `json:"-" db:"password"`                          // 密码（不返回给前端）
	MemberLevel     *int       `json:"member_level" db:"member_level"`           // 会员等级（默认0，可空）
	MemberExpireAt  *int64     `json:"member_expire_at" db:"member_expire_at"`   // 会员到期时间（10位时间戳，可空）
	InviteCode      string     `json:"invite_code" db:"invite_code"`             // 邀请码
	CreatedAt       time.Time  `json:"created_at" db:"created_at"`               // 创建时间
	UpdatedAt       time.Time  `json:"updated_at" db:"updated_at"`               // 更新时间
}

// MT4Account MT4账户表
type MT4Account struct {
	ID              int64      `json:"id" db:"id"`                               // 主键
	UserID          int64      `json:"user_id" db:"user_id"`                     // 用户id
	PlatformID      int64      `json:"platform_id" db:"platform_id"`             // 平台id
	Account         string     `json:"account" db:"account"`                     // 账户
	Password        string     `json:"password" db:"password"`                   // 密码
	Type            *int       `json:"type" db:"type"`                           // 类型（0美元，1美分，可空，默认0）
	Amount          *float64   `json:"amount" db:"amount"`                       // 金额（可空默认0.00）
	Profit          *float64   `json:"profit" db:"profit"`                       // 收益（可空默认0.00）
	Status          int        `json:"status" db:"status"`                       // 状态（默认0）
	Remark          *string    `json:"remark" db:"remark"`                       // 备注（可空字符）
	DeletedAt       *time.Time `json:"deleted_at,omitempty" db:"deleted_at"`     // 删除时间（软删除）
	CreatedAt       time.Time  `json:"created_at" db:"created_at"`               // 创建时间
	UpdatedAt       time.Time  `json:"updated_at" db:"updated_at"`               // 更新时间
}

// EA EA表
type EA struct {
	ID              int64      `json:"id" db:"id"`                               // 主键
	Name            string     `json:"name" db:"name"`                           // EA名字
	Type            *string    `json:"type" db:"type"`                           // EA类型（字符串可空）
	Profit          *string    `json:"profit" db:"profit"`                       // 收益（字符串可空）
	Description     *string    `json:"description" db:"description"`             // 描述（字符串可空）
	Status          *int       `json:"status" db:"status"`                       // 状态（整数可空，默认0）
	Sort            *int       `json:"sort" db:"sort"`                           // 排序（整数可空，默认0）
	CreatedAt       time.Time  `json:"created_at" db:"created_at"`               // 创建时间
	UpdatedAt       time.Time  `json:"updated_at" db:"updated_at"`               // 更新时间
}

// EAParam EA参数表
type EAParam struct {
	ID              int64      `json:"id" db:"id"`                               // 主键
	EAID            int64      `json:"ea_id" db:"ea_id"`                         // EA ID
	Name            string     `json:"name" db:"name"`                           // 参数名称
	Label           string     `json:"label" db:"label"`                         // 参数标签
	Type            string     `json:"type" db:"type"`                           // 类型（对应前端的表单类型比如number等）
	DefaultValue    *string    `json:"default_value" db:"default_value"`         // 默认值（可空）
	Min             *float64   `json:"min" db:"min"`                             // 最小值（可空）
	Max             *float64   `json:"max" db:"max"`                             // 最大值（可空）
	Required        *int       `json:"required" db:"required"`                   // 是否必填（默认0，可空）
	CreatedAt       time.Time  `json:"created_at" db:"created_at"`               // 创建时间
	UpdatedAt       time.Time  `json:"updated_at" db:"updated_at"`               // 更新时间
}

// Order 订单表
type Order struct {
	ID              int64      `json:"id" db:"id"`                               // 主键
	UserID          int64      `json:"user_id" db:"user_id"`                     // 用户ID
	EAID            int64      `json:"ea_id" db:"ea_id"`                         // EA ID
	MT4AccountID    int64      `json:"mt4_account_id" db:"mt4_account_id"`       // MT4账户ID
	Symbol          string     `json:"symbol" db:"symbol"`                       // 货币对
	Status          int        `json:"status" db:"status"`                       // 状态（0运行中，1已停止等）
	Params          *string    `json:"params" db:"params"`                       // EA参数（JSON字符串存储）
	DeletedAt       *time.Time `json:"deleted_at,omitempty" db:"deleted_at"`     // 删除时间（软删除）
	CreatedAt       time.Time  `json:"created_at" db:"created_at"`               // 创建时间
	UpdatedAt       time.Time  `json:"updated_at" db:"updated_at"`               // 更新时间
}

// OrderList 订单列表表（MT4订单详情）
type OrderList struct {
	ID              int64      `json:"id" db:"id"`                               // 主键
	OrderID         int64      `json:"order_id" db:"order_id"`                   // 对应的所属order表的id
	Ticket          int64      `json:"ticket" db:"ticket"`                       // MT4订单号
	OpenTime        *time.Time `json:"open_time" db:"open_time"`                 // 开仓时间
	CloseTime       *time.Time `json:"close_time" db:"close_time"`               // 平仓时间
	Symbol          string     `json:"symbol" db:"symbol"`                       // 货币对
	Type            int        `json:"type" db:"type"`                           // 订单类型（0买入，1卖出等）
	Lots            float64    `json:"lots" db:"lots"`                           // 手数
	OpenPrice       float64    `json:"open_price" db:"open_price"`               // 开仓价
	ClosePrice      *float64   `json:"close_price" db:"close_price"`             // 平仓价
	StopLoss        *float64   `json:"stop_loss" db:"stop_loss"`                 // 止损
	TakeProfit      *float64   `json:"take_profit" db:"take_profit"`             // 止盈
	MagicNumber     *int64     `json:"magic_number" db:"magic_number"`           // 魔术号
	Swap            *float64   `json:"swap" db:"swap"`                           // 库存费
	Commission      *float64   `json:"commission" db:"commission"`               // 手续费
	Profit          *float64   `json:"profit" db:"profit"`                       // 盈亏
	Status          int        `json:"status" db:"status"`                       // 状态
	CreatedAt       time.Time  `json:"created_at" db:"created_at"`               // 创建时间
	UpdatedAt       time.Time  `json:"updated_at" db:"updated_at"`               // 更新时间
}

// Platform 平台表（券商）
type Platform struct {
	ID              int64      `json:"id" db:"id"`                               // 主键
	ParentID        *int64     `json:"parent_id" db:"parent_id"`                 // 上级id（可空，默认0）
	Title           string     `json:"title" db:"title"`                         // 券商名
	Status          *int       `json:"status" db:"status"`                       // 状态（可空默认0）
	Server          *int       `json:"server" db:"server"`                       // 所属的服务器id（整数，可空）
	Remark          *string    `json:"remark" db:"remark"`                       // 备注（可空）
	CreatedAt       time.Time  `json:"created_at" db:"created_at"`               // 创建时间
	UpdatedAt       time.Time  `json:"updated_at" db:"updated_at"`               // 更新时间
}

// Symbol 货币对表
type Symbol struct {
	ID              int64      `json:"id" db:"id"`                               // 主键
	Title           string     `json:"title" db:"title"`                         // 货币对名字
	Sort            *int       `json:"sort" db:"sort"`                           // 排序（默认0可空）
	Status          *int       `json:"status" db:"status"`                       // 状态（默认0可空）
	CreatedAt       time.Time  `json:"created_at" db:"created_at"`               // 创建时间
	UpdatedAt       time.Time  `json:"updated_at" db:"updated_at"`               // 更新时间
}
