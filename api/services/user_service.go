package services

import (
	"api/models"
	"database/sql"
	"fmt"

	"github.com/jmoiron/sqlx"
)

// UserService 用户服务
type UserService struct {
	db *sqlx.DB
}

// NewUserService 创建用户服务
func NewUserService(db *sqlx.DB) *UserService {
	return &UserService{db: db}
}

// GetUserByEmail 根据邮箱获取用户
func (s *UserService) GetUserByEmail(email string) (*models.User, error) {
	var user models.User
	query := `
		SELECT id, email, nickname, password, member_level, member_expire_at, 
		       invite_code, created_at, updated_at
		FROM users
		WHERE email = ?
	`
	err := s.db.Get(&user, query, email)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("用户不存在")
		}
		return nil, err
	}
	return &user, nil
}

// GetUserByID 根据ID获取用户
func (s *UserService) GetUserByID(id int64) (*models.User, error) {
	var user models.User
	query := `
		SELECT id, email, nickname, password, member_level, member_expire_at, 
		       invite_code, created_at, updated_at
		FROM users
		WHERE id = ?
	`
	err := s.db.Get(&user, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("用户不存在")
		}
		return nil, err
	}
	return &user, nil
}

// CreateUser 创建用户
func (s *UserService) CreateUser(user *models.User) error {
	query := `
		INSERT INTO users (email, nickname, password, member_level, member_expire_at, 
		                   invite_code, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`
	result, err := s.db.Exec(
		query,
		user.Email,
		user.Nickname,
		user.Password,
		user.MemberLevel,
		user.MemberExpireAt,
		user.InviteCode,
		user.CreatedAt,
		user.UpdatedAt,
	)

	if err != nil {
		return err
	}

	// 获取插入的ID
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	user.ID = id

	return nil
}

// UpdateUser 更新用户
func (s *UserService) UpdateUser(user *models.User) error {
	query := `
		UPDATE users
		SET email = ?, nickname = ?, password = ?, member_level = ?, 
		    member_expire_at = ?, updated_at = ?
		WHERE id = ?
	`
	result, err := s.db.Exec(
		query,
		user.Email,
		user.Nickname,
		user.Password,
		user.MemberLevel,
		user.MemberExpireAt,
		user.UpdatedAt,
		user.ID,
	)

	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return fmt.Errorf("用户不存在")
	}

	return nil
}

// EmailExists 检查邮箱是否存在
func (s *UserService) EmailExists(email string) (bool, error) {
	var count int
	query := `SELECT COUNT(*) FROM users WHERE email = ?`
	err := s.db.Get(&count, query, email)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// InviteCodeExists 检查邀请码是否存在
func (s *UserService) InviteCodeExists(code string) (bool, error) {
	var count int
	query := `SELECT COUNT(*) FROM users WHERE invite_code = ?`
	err := s.db.Get(&count, query, code)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
