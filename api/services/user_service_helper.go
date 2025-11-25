package services

import (
	"api/models"
	"database/sql"
	"fmt"
	"log"
)

// GetUserByID 根据ID获取用户信息
func (s *MT4Service) GetUserByID(userID int64) (*models.User, error) {
	var user models.User
	query := `
		SELECT id, email, nickname, password, invite_code, inviter_id, 
		       member_level, member_expire_at, created_at, updated_at
		FROM users
		WHERE id = ?
	`
	err := s.db.Get(&user, query, userID)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("WARN: User not found: id=%d", userID)
			return nil, fmt.Errorf("用户不存在")
		}
		log.Printf("ERROR: Failed to get user by id=%d: %v", userID, err)
		return nil, err
	}
	return &user, nil
}
