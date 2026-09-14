// Package repository 提供用户数据访问层
package repository

import (
	"context"

	"gorm.io/gorm"

	"github.com/kerbos/ticketdesk/internal/model"
)

// UserRepository 用户数据访问接口
type UserRepository interface {
	Create(ctx context.Context, user *model.User) error
	GetByID(ctx context.Context, id uint64) (*model.User, error)
	GetByIDs(ctx context.Context, ids []uint64) ([]*model.User, error)
	GetByUsername(ctx context.Context, username string) (*model.User, error)
	GetByEmail(ctx context.Context, email string) (*model.User, error)
	GetByResetToken(ctx context.Context, token string) (*model.User, error)
	GetBySSOSubject(ctx context.Context, provider, subject string) (*model.User, error)
	Update(ctx context.Context, user *model.User) error
	Delete(ctx context.Context, id uint64) error
	List(ctx context.Context, offset, limit int, keyword string, status *int8) ([]*model.User, int64, error)
	ExistsByUsername(ctx context.Context, username string) (bool, error)
	ExistsByEmail(ctx context.Context, email string) (bool, error)
}

// userRepository 用户数据访问实现
type userRepository struct {
	db *gorm.DB
}

// NewUserRepository 创建用户数据访问实例
func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepository{db: db}
}

// Create 创建用户
func (r *userRepository) Create(ctx context.Context, user *model.User) error {
	return r.db.WithContext(ctx).Create(user).Error
}

// GetByID 根据 ID 获取用户
func (r *userRepository) GetByID(ctx context.Context, id uint64) (*model.User, error) {
	var user model.User
	err := r.db.WithContext(ctx).First(&user, id).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// GetByIDs 根据 ID 列表批量获取用户
func (r *userRepository) GetByIDs(ctx context.Context, ids []uint64) ([]*model.User, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var users []*model.User
	err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&users).Error
	return users, err
}

// GetByUsername 根据用户名获取用户
func (r *userRepository) GetByUsername(ctx context.Context, username string) (*model.User, error) {
	var user model.User
	err := r.db.WithContext(ctx).Where("username = ?", username).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// GetByEmail 根据邮箱获取用户
func (r *userRepository) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	var user model.User
	err := r.db.WithContext(ctx).Where("email = ?", email).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// GetByResetToken 根据重置密码令牌获取用户
func (r *userRepository) GetByResetToken(ctx context.Context, token string) (*model.User, error) {
	var user model.User
	err := r.db.WithContext(ctx).Where("reset_password_token = ?", token).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// GetBySSOSubject 根据 SSO provider 和 subject 获取用户
func (r *userRepository) GetBySSOSubject(ctx context.Context, provider, subject string) (*model.User, error) {
	var user model.User
	err := r.db.WithContext(ctx).Where("sso_provider = ? AND sso_subject = ?", provider, subject).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// Update 更新用户
func (r *userRepository) Update(ctx context.Context, user *model.User) error {
	return r.db.WithContext(ctx).Save(user).Error
}

// Delete 硬删除用户
func (r *userRepository) Delete(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Unscoped().Delete(&model.User{}, id).Error
}

// List 分页查询用户列表
func (r *userRepository) List(ctx context.Context, offset, limit int, keyword string, status *int8) ([]*model.User, int64, error) {
	var users []*model.User
	var total int64

	query := r.db.WithContext(ctx).Model(&model.User{})

	// 关键字搜索
	if keyword != "" {
		query = query.Where("username LIKE ? OR email LIKE ? OR display_name LIKE ?",
			"%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%")
	}

	// 状态过滤
	if status != nil {
		query = query.Where("status = ?", *status)
	}

	// 统计总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页查询
	if err := query.Offset(offset).Limit(limit).Order("id DESC").Find(&users).Error; err != nil {
		return nil, 0, err
	}

	return users, total, nil
}

// ExistsByUsername 检查用户名是否存在
func (r *userRepository) ExistsByUsername(ctx context.Context, username string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.User{}).Where("username = ?", username).Count(&count).Error
	return count > 0, err
}

// ExistsByEmail 检查邮箱是否存在
func (r *userRepository) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.User{}).Where("email = ?", email).Count(&count).Error
	return count > 0, err
}

// RoleRepository 角色数据访问接口
type RoleRepository interface {
	GetByID(ctx context.Context, id uint64) (*model.Role, error)
	GetByName(ctx context.Context, name string) (*model.Role, error)
	List(ctx context.Context) ([]*model.Role, error)
}

// roleRepository 角色数据访问实现
type roleRepository struct {
	db *gorm.DB
}

// NewRoleRepository 创建角色数据访问实例
func NewRoleRepository(db *gorm.DB) RoleRepository {
	return &roleRepository{db: db}
}

// GetByID 根据 ID 获取角色
func (r *roleRepository) GetByID(ctx context.Context, id uint64) (*model.Role, error) {
	var role model.Role
	err := r.db.WithContext(ctx).First(&role, id).Error
	if err != nil {
		return nil, err
	}
	return &role, nil
}

// GetByName 根据名称获取角色
func (r *roleRepository) GetByName(ctx context.Context, name string) (*model.Role, error) {
	var role model.Role
	err := r.db.WithContext(ctx).Where("name = ?", name).First(&role).Error
	if err != nil {
		return nil, err
	}
	return &role, nil
}

// List 获取所有角色
func (r *roleRepository) List(ctx context.Context) ([]*model.Role, error) {
	var roles []*model.Role
	err := r.db.WithContext(ctx).Find(&roles).Error
	return roles, err
}

// UserRoleRepository 用户角色关联数据访问接口
type UserRoleRepository interface {
	Create(ctx context.Context, userRole *model.UserRole) error
	Delete(ctx context.Context, userID, roleID uint64) error
	GetUserRoles(ctx context.Context, userID uint64) ([]*model.Role, error)
	GetUserRoleNames(ctx context.Context, userID uint64) ([]string, error)
	// GetRoleNamesByUserIDs 批量获取多个用户的角色名，避免列表场景 N+1
	GetRoleNamesByUserIDs(ctx context.Context, userIDs []uint64) (map[uint64][]string, error)
	HasRole(ctx context.Context, userID uint64, roleName string) (bool, error)
	AssignRole(ctx context.Context, userID uint64, roleName string) error
}

// userRoleRepository 用户角色关联数据访问实现
type userRoleRepository struct {
	db *gorm.DB
}

// NewUserRoleRepository 创建用户角色关联数据访问实例
func NewUserRoleRepository(db *gorm.DB) UserRoleRepository {
	return &userRoleRepository{db: db}
}

// Create 创建用户角色关联
func (r *userRoleRepository) Create(ctx context.Context, userRole *model.UserRole) error {
	return r.db.WithContext(ctx).Create(userRole).Error
}

// Delete 硬删除用户角色关联
func (r *userRoleRepository) Delete(ctx context.Context, userID, roleID uint64) error {
	return r.db.WithContext(ctx).Unscoped().Where("user_id = ? AND role_id = ?", userID, roleID).Delete(&model.UserRole{}).Error
}

// GetUserRoles 获取用户的所有角色
func (r *userRoleRepository) GetUserRoles(ctx context.Context, userID uint64) ([]*model.Role, error) {
	var roles []*model.Role
	err := r.db.WithContext(ctx).
		Table("roles").
		Joins("INNER JOIN user_roles ON roles.id = user_roles.role_id").
		Where("user_roles.user_id = ?", userID).
		Find(&roles).Error
	return roles, err
}

// GetRoleNamesByUserIDs 批量获取多个用户的角色名称
//
// 用户列表原先对每条记录单独查一次角色：一页 20 条就是 1 + 20 次查询。
// 这里一次 JOIN 取回全部，再在内存里按用户分组。
func (r *userRoleRepository) GetRoleNamesByUserIDs(ctx context.Context, userIDs []uint64) (map[uint64][]string, error) {
	result := make(map[uint64][]string, len(userIDs))
	if len(userIDs) == 0 {
		return result, nil
	}

	var rows []struct {
		UserID uint64
		Name   string
	}
	err := r.db.WithContext(ctx).
		Table("roles").
		Select("user_roles.user_id AS user_id, roles.name AS name").
		Joins("INNER JOIN user_roles ON roles.id = user_roles.role_id").
		Where("user_roles.user_id IN ?", userIDs).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	for _, row := range rows {
		result[row.UserID] = append(result[row.UserID], row.Name)
	}
	return result, nil
}

// GetUserRoleNames 获取用户的所有角色名称
func (r *userRoleRepository) GetUserRoleNames(ctx context.Context, userID uint64) ([]string, error) {
	var roleNames []string
	err := r.db.WithContext(ctx).
		Table("roles").
		Select("roles.name").
		Joins("INNER JOIN user_roles ON roles.id = user_roles.role_id").
		Where("user_roles.user_id = ?", userID).
		Pluck("name", &roleNames).Error
	return roleNames, err
}

// HasRole 检查用户是否拥有指定角色
func (r *userRoleRepository) HasRole(ctx context.Context, userID uint64, roleName string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Table("user_roles").
		Joins("INNER JOIN roles ON roles.id = user_roles.role_id").
		Where("user_roles.user_id = ? AND roles.name = ?", userID, roleName).
		Count(&count).Error
	return count > 0, err
}

// AssignRole 为用户分配角色
func (r *userRoleRepository) AssignRole(ctx context.Context, userID uint64, roleName string) error {
	var role model.Role
	if err := r.db.WithContext(ctx).Where("name = ?", roleName).First(&role).Error; err != nil {
		return err
	}

	userRole := &model.UserRole{
		UserID: userID,
		RoleID: role.ID,
	}
	return r.db.WithContext(ctx).Create(userRole).Error
}
