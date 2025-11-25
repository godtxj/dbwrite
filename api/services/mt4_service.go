package services

import (
	"api/models"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/jmoiron/sqlx"
)

// MT4Service MT4服务
type MT4Service struct {
	db *sqlx.DB
}

// NewMT4Service 创建MT4服务
func NewMT4Service(db *sqlx.DB) *MT4Service {
	return &MT4Service{db: db}
}

// ==================== 平台管理 ====================

// GetPlatforms 获取所有平台（包含层级关系）
func (s *MT4Service) GetPlatforms() ([]*models.Platform, error) {
	var platforms []*models.Platform
	query := `
		SELECT id, parent_id, title, status, server, remark, created_at, updated_at
		FROM platforms
		ORDER BY parent_id, id
	`
	err := s.db.Select(&platforms, query)
	if err != nil {
		log.Printf("ERROR: Failed to get platforms: %v", err)
	}
	return platforms, err
}

// GetTopLevelPlatforms 获取顶级平台（parent_id = 0 或 NULL）
func (s *MT4Service) GetTopLevelPlatforms() ([]*models.Platform, error) {
	var platforms []*models.Platform
	query := `
		SELECT id, parent_id, title, status, server, remark, created_at, updated_at
		FROM platforms
		WHERE parent_id = 0 OR parent_id IS NULL
		ORDER BY id
	`
	err := s.db.Select(&platforms, query)
	if err != nil {
		log.Printf("ERROR: Failed to get top level platforms: %v", err)
	}
	return platforms, err
}

// GetSubPlatforms 获取指定平台的下级平台
func (s *MT4Service) GetSubPlatforms(parentID int64) ([]*models.Platform, error) {
	var platforms []*models.Platform
	query := `
		SELECT id, parent_id, title, status, server, remark, created_at, updated_at
		FROM platforms
		WHERE parent_id = ?
		ORDER BY id
	`
	err := s.db.Select(&platforms, query, parentID)
	if err != nil {
		log.Printf("ERROR: Failed to get sub platforms for parent_id=%d: %v", parentID, err)
	}
	return platforms, err
}

// GetPlatformByID 根据ID获取平台
func (s *MT4Service) GetPlatformByID(id int64) (*models.Platform, error) {
	var platform models.Platform
	query := `
		SELECT id, parent_id, title, status, server, remark, created_at, updated_at
		FROM platforms
		WHERE id = ?
	`
	err := s.db.Get(&platform, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("WARN: Platform not found: id=%d", id)
			return nil, fmt.Errorf("平台不存在")
		}
		log.Printf("ERROR: Failed to get platform by id=%d: %v", id, err)
		return nil, err
	}
	return &platform, nil
}

// ==================== MT4账户管理 ====================

// GetMT4AccountsByUserID 获取用户的所有MT4账户（支持分页）
func (s *MT4Service) GetMT4AccountsByUserID(userID int64, limit, offset int) ([]*models.MT4Account, error) {
	var accounts []*models.MT4Account
	query := `
		SELECT id, user_id, platform_id, account, password, type, amount, profit, 
		       status, remark, deleted_at, created_at, updated_at
		FROM mt4_accounts
		WHERE user_id = ? AND deleted_at IS NULL
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`
	err := s.db.Select(&accounts, query, userID, limit, offset)
	if err != nil {
		log.Printf("ERROR: Failed to get MT4 accounts for user_id=%d: %v", userID, err)
	}
	return accounts, err
}

// CountMT4AccountsByUserID 统计用户的MT4账户总数
func (s *MT4Service) CountMT4AccountsByUserID(userID int64) (int, error) {
	var count int
	query := `SELECT COUNT(*) FROM mt4_accounts WHERE user_id = ? AND deleted_at IS NULL`
	err := s.db.Get(&count, query, userID)
	if err != nil {
		log.Printf("ERROR: Failed to count MT4 accounts for user_id=%d: %v", userID, err)
	}
	return count, err
}

// GetMT4AccountByID 根据ID获取MT4账户
func (s *MT4Service) GetMT4AccountByID(id int64) (*models.MT4Account, error) {
	var account models.MT4Account
	query := `
		SELECT id, user_id, platform_id, account, password, type, amount, profit, 
		       status, remark, deleted_at, created_at, updated_at
		FROM mt4_accounts
		WHERE id = ? AND deleted_at IS NULL
	`
	err := s.db.Get(&account, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("WARN: MT4 account not found or deleted: id=%d", id)
			return nil, fmt.Errorf("MT4账户不存在")
		}
		log.Printf("ERROR: Failed to get MT4 account by id=%d: %v", id, err)
		return nil, err
	}
	return &account, nil
}

// CreateMT4Account 创建MT4账户
func (s *MT4Service) CreateMT4Account(account *models.MT4Account) error {
	query := `
		INSERT INTO mt4_accounts (user_id, platform_id, account, password, type, 
		                          amount, profit, status, remark, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	result, err := s.db.Exec(
		query,
		account.UserID,
		account.PlatformID,
		account.Account,
		account.Password,
		account.Type,
		account.Amount,
		account.Profit,
		account.Status,
		account.Remark,
		account.CreatedAt,
		account.UpdatedAt,
	)

	if err != nil {
		log.Printf("ERROR: Failed to create MT4 account for user_id=%d: %v", account.UserID, err)
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		log.Printf("ERROR: Failed to get last insert id: %v", err)
		return err
	}
	account.ID = id
	log.Printf("INFO: MT4 account created successfully: id=%d, user_id=%d", account.ID, account.UserID)

	return nil
}

// UpdateMT4Account 更新MT4账户
func (s *MT4Service) UpdateMT4Account(account *models.MT4Account) error {
	query := `
		UPDATE mt4_accounts
		SET platform_id = ?, account = ?, password = ?, type = ?, 
		    amount = ?, profit = ?, status = ?, remark = ?, updated_at = ?
		WHERE id = ? AND deleted_at IS NULL
	`
	result, err := s.db.Exec(
		query,
		account.PlatformID,
		account.Account,
		account.Password,
		account.Type,
		account.Amount,
		account.Profit,
		account.Status,
		account.Remark,
		account.UpdatedAt,
		account.ID,
	)

	if err != nil {
		log.Printf("ERROR: Failed to update MT4 account id=%d: %v", account.ID, err)
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		log.Printf("ERROR: Failed to get rows affected: %v", err)
		return err
	}

	if rows == 0 {
		log.Printf("WARN: MT4 account not found or already deleted: id=%d", account.ID)
		return fmt.Errorf("MT4账户不存在")
	}

	log.Printf("INFO: MT4 account updated successfully: id=%d", account.ID)
	return nil
}

// DeleteMT4Account 软删除MT4账户
func (s *MT4Service) DeleteMT4Account(id int64) error {
	query := `UPDATE mt4_accounts SET deleted_at = ? WHERE id = ? AND deleted_at IS NULL`
	result, err := s.db.Exec(query, time.Now(), id)
	if err != nil {
		log.Printf("ERROR: Failed to soft delete MT4 account id=%d: %v", id, err)
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		log.Printf("ERROR: Failed to get rows affected: %v", err)
		return err
	}

	if rows == 0 {
		log.Printf("WARN: MT4 account not found or already deleted: id=%d", id)
		return fmt.Errorf("MT4账户不存在")
	}

	log.Printf("INFO: MT4 account soft deleted successfully: id=%d", id)
	return nil
}

// CheckMT4AccountOwner 检查MT4账户是否属于指定用户
func (s *MT4Service) CheckMT4AccountOwner(accountID, userID int64) (bool, error) {
	var count int
	query := `SELECT COUNT(*) FROM mt4_accounts WHERE id = ? AND user_id = ? AND deleted_at IS NULL`
	err := s.db.Get(&count, query, accountID, userID)
	if err != nil {
		log.Printf("ERROR: Failed to check MT4 account owner: account_id=%d, user_id=%d: %v", accountID, userID, err)
		return false, err
	}
	return count > 0, nil
}

// ==================== EA管理 ====================

// GetEAs 获取所有EA（支持分页）
func (s *MT4Service) GetEAs(limit, offset int) ([]*models.EA, error) {
	var eas []*models.EA
	query := `
		SELECT id, name, type, profit, description, status, sort, created_at, updated_at
		FROM eas
		WHERE status = 1
		ORDER BY sort, id
		LIMIT ? OFFSET ?
	`
	err := s.db.Select(&eas, query, limit, offset)
	if err != nil {
		log.Printf("ERROR: Failed to get EAs: %v", err)
	}
	return eas, err
}

// CountEAs 统计EA总数
func (s *MT4Service) CountEAs() (int, error) {
	var count int
	query := `SELECT COUNT(*) FROM eas WHERE status = 1`
	err := s.db.Get(&count, query)
	if err != nil {
		log.Printf("ERROR: Failed to count EAs: %v", err)
	}
	return count, err
}

// GetEAByID 根据ID获取EA
func (s *MT4Service) GetEAByID(id int64) (*models.EA, error) {
	var ea models.EA
	query := `
		SELECT id, name, type, profit, description, status, sort, created_at, updated_at
		FROM eas
		WHERE id = ?
	`
	err := s.db.Get(&ea, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("WARN: EA not found: id=%d", id)
			return nil, fmt.Errorf("EA不存在")
		}
		log.Printf("ERROR: Failed to get EA by id=%d: %v", id, err)
		return nil, err
	}
	return &ea, nil
}

// GetEAParams 获取EA的参数列表
func (s *MT4Service) GetEAParams(eaID int64) ([]*models.EAParam, error) {
	var params []*models.EAParam
	query := `
		SELECT id, ea_id, name, label, type, default_value, min, max, required, 
		       created_at, updated_at
		FROM ea_params
		WHERE ea_id = ?
		ORDER BY id
	`
	err := s.db.Select(&params, query, eaID)
	if err != nil {
		log.Printf("ERROR: Failed to get EA params for ea_id=%d: %v", eaID, err)
	}
	return params, err
}

// ==================== 订单管理 ====================

// GetUserOrders 获取用户的所有订单（支持分页）
func (s *MT4Service) GetUserOrders(userID int64, limit, offset int) ([]*models.Order, error) {
	var orders []*models.Order
	query := `
		SELECT id, user_id, ea_id, mt4_account_id, symbol, status, params, 
		       deleted_at, created_at, updated_at
		FROM orders
		WHERE user_id = ? AND deleted_at IS NULL
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`
	err := s.db.Select(&orders, query, userID, limit, offset)
	if err != nil {
		log.Printf("ERROR: Failed to get orders for user_id=%d: %v", userID, err)
	}
	return orders, err
}

// CountUserOrders 统计用户的订单总数
func (s *MT4Service) CountUserOrders(userID int64) (int, error) {
	var count int
	query := `SELECT COUNT(*) FROM orders WHERE user_id = ? AND deleted_at IS NULL`
	err := s.db.Get(&count, query, userID)
	if err != nil {
		log.Printf("ERROR: Failed to count orders for user_id=%d: %v", userID, err)
	}
	return count, err
}

// GetOrderByID 根据ID获取订单
func (s *MT4Service) GetOrderByID(id int64) (*models.Order, error) {
	var order models.Order
	query := `
		SELECT id, user_id, ea_id, mt4_account_id, symbol, status, params, 
		       deleted_at, created_at, updated_at
		FROM orders
		WHERE id = ? AND deleted_at IS NULL
	`
	err := s.db.Get(&order, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("WARN: Order not found or deleted: id=%d", id)
			return nil, fmt.Errorf("订单不存在")
		}
		log.Printf("ERROR: Failed to get order by id=%d: %v", id, err)
		return nil, err
	}
	return &order, nil
}

// CreateOrder 创建订单（启动EA）- 使用事务
func (s *MT4Service) CreateOrder(order *models.Order) error {
	// 开始事务
	tx, err := s.db.Beginx()
	if err != nil {
		log.Printf("ERROR: Failed to begin transaction: %v", err)
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
			log.Printf("INFO: Transaction rolled back")
		}
	}()

	// 使用FOR UPDATE锁定行，防止并发创建重复订单
	var count int
	checkQuery := `
		SELECT COUNT(*) FROM orders 
		WHERE user_id = ? AND ea_id = ? AND symbol = ? AND status = 0 AND deleted_at IS NULL
		FOR UPDATE
	`
	err = tx.Get(&count, checkQuery, order.UserID, order.EAID, order.Symbol)
	if err != nil {
		log.Printf("ERROR: Failed to check existing orders: %v", err)
		return err
	}
	if count > 0 {
		log.Printf("WARN: Duplicate order attempt: user_id=%d, ea_id=%d, symbol=%s", order.UserID, order.EAID, order.Symbol)
		return fmt.Errorf("该EA在此品种上已有运行中的订单")
	}

	// 插入订单
	query := `
		INSERT INTO orders (user_id, ea_id, mt4_account_id, symbol, status, params, 
		                    created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`
	result, err := tx.Exec(
		query,
		order.UserID,
		order.EAID,
		order.MT4AccountID,
		order.Symbol,
		order.Status,
		order.Params,
		order.CreatedAt,
		order.UpdatedAt,
	)

	if err != nil {
		log.Printf("ERROR: Failed to insert order: %v", err)
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		log.Printf("ERROR: Failed to get last insert id: %v", err)
		return err
	}
	order.ID = id

	// 提交事务
	if err = tx.Commit(); err != nil {
		log.Printf("ERROR: Failed to commit transaction: %v", err)
		return err
	}

	log.Printf("INFO: Order created successfully: id=%d, user_id=%d, ea_id=%d, symbol=%s", order.ID, order.UserID, order.EAID, order.Symbol)
	return nil
}

// UpdateOrderStatus 更新订单状态（暂停/恢复EA）
func (s *MT4Service) UpdateOrderStatus(id int64, status int) error {
	query := `UPDATE orders SET status = ?, updated_at = NOW() WHERE id = ? AND deleted_at IS NULL`
	result, err := s.db.Exec(query, status, id)
	if err != nil {
		log.Printf("ERROR: Failed to update order status: id=%d, status=%d: %v", id, status, err)
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		log.Printf("ERROR: Failed to get rows affected: %v", err)
		return err
	}

	if rows == 0 {
		log.Printf("WARN: Order not found or already deleted: id=%d", id)
		return fmt.Errorf("订单不存在")
	}

	log.Printf("INFO: Order status updated: id=%d, status=%d", id, status)
	return nil
}

// DeleteOrder 软删除订单（删除EA）
func (s *MT4Service) DeleteOrder(id int64) error {
	query := `UPDATE orders SET deleted_at = ? WHERE id = ? AND deleted_at IS NULL`
	result, err := s.db.Exec(query, time.Now(), id)
	if err != nil {
		log.Printf("ERROR: Failed to soft delete order id=%d: %v", id, err)
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		log.Printf("ERROR: Failed to get rows affected: %v", err)
		return err
	}

	if rows == 0 {
		log.Printf("WARN: Order not found or already deleted: id=%d", id)
		return fmt.Errorf("订单不存在")
	}

	log.Printf("INFO: Order soft deleted successfully: id=%d", id)
	return nil
}

// CheckOrderOwner 检查订单是否属于指定用户
func (s *MT4Service) CheckOrderOwner(orderID, userID int64) (bool, error) {
	var count int
	query := `SELECT COUNT(*) FROM orders WHERE id = ? AND user_id = ? AND deleted_at IS NULL`
	err := s.db.Get(&count, query, orderID, userID)
	if err != nil {
		log.Printf("ERROR: Failed to check order owner: order_id=%d, user_id=%d: %v", orderID, userID, err)
		return false, err
	}
	return count > 0, nil
}

// ==================== 订单列表管理 ====================

// GetOrderList 获取订单的交易列表（支持分页）
func (s *MT4Service) GetOrderList(orderID int64, limit, offset int) ([]*models.OrderList, error) {
	var orderList []*models.OrderList
	query := `
		SELECT id, order_id, ticket, open_time, close_time, symbol, type, lots, 
		       open_price, close_price, stop_loss, take_profit, magic_number, 
		       swap, commission, profit, status, created_at, updated_at
		FROM order_list
		WHERE order_id = ?
		ORDER BY open_time DESC
		LIMIT ? OFFSET ?
	`
	err := s.db.Select(&orderList, query, orderID, limit, offset)
	if err != nil {
		log.Printf("ERROR: Failed to get order list for order_id=%d: %v", orderID, err)
	}
	return orderList, err
}

// CountOrderList 统计订单的交易总数
func (s *MT4Service) CountOrderList(orderID int64) (int, error) {
	var count int
	query := `SELECT COUNT(*) FROM order_list WHERE order_id = ?`
	err := s.db.Get(&count, query, orderID)
	if err != nil {
		log.Printf("ERROR: Failed to count order list for order_id=%d: %v", orderID, err)
	}
	return count, err
}

// GetSymbols 获取所有货币对（支持分页）
func (s *MT4Service) GetSymbols(limit, offset int) ([]*models.Symbol, error) {
	var symbols []*models.Symbol
	query := `
		SELECT id, title, sort, status, created_at, updated_at
		FROM symbols
		WHERE status = 1
		ORDER BY sort, id
		LIMIT ? OFFSET ?
	`
	err := s.db.Select(&symbols, query, limit, offset)
	if err != nil {
		log.Printf("ERROR: Failed to get symbols: %v", err)
	}
	return symbols, err
}

// CountSymbols 统计货币对总数
func (s *MT4Service) CountSymbols() (int, error) {
	var count int
	query := `SELECT COUNT(*) FROM symbols WHERE status = 1`
	err := s.db.Get(&count, query)
	if err != nil {
		log.Printf("ERROR: Failed to count symbols: %v", err)
	}
	return count, err
}
