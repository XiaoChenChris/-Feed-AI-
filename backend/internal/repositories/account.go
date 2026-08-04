package repositories

import (
	"context"

	"feedsystem_ai_go/internal/models"
	"gorm.io/gorm"
)

type AccountRepository struct {
	DB *gorm.DB
}

func NewAccountRepository(db *gorm.DB) *AccountRepository {
	return &AccountRepository{DB: db}
}

func (ar *AccountRepository) CreateAccount(ctx context.Context, account *models.Account) error {
	return ar.DB.WithContext(ctx).Create(account).Error
}

func (ar *AccountRepository) Rename(ctx context.Context, id uint, newUsername string) error {
	result := ar.DB.WithContext(ctx).Model(&models.Account{}).Where("id = ?", id).Update("username", newUsername)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (ar *AccountRepository) RenameWithToken(ctx context.Context, id uint, newUsername string, token string) error {
	return ar.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&models.Account{}).Where("id = ?", id).Update("username", newUsername)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return tx.Model(&models.Account{}).Where("id = ?", id).Update("token", token).Error
	})
}

func (ar *AccountRepository) ChangePassword(ctx context.Context, id uint, newPassword string) error {
	return ar.DB.WithContext(ctx).Model(&models.Account{}).Where("id = ?", id).Update("password", newPassword).Error
}

func (ar *AccountRepository) FindByID(ctx context.Context, id uint) (*models.Account, error) {
	var account models.Account
	if err := ar.DB.WithContext(ctx).First(&account, id).Error; err != nil {
		return nil, err
	}
	return &account, nil
}

func (ar *AccountRepository) FindByUsername(ctx context.Context, username string) (*models.Account, error) {
	var account models.Account
	if err := ar.DB.WithContext(ctx).Where("username = ?", username).First(&account).Error; err != nil {
		return nil, err
	}
	return &account, nil
}

func (ar *AccountRepository) Login(ctx context.Context, id uint, token, refreshToken string) error {
	return ar.DB.WithContext(ctx).Model(&models.Account{}).Where("id = ?", id).Updates(map[string]interface{}{"token": token, "refresh_token": refreshToken}).Error
}

func (ar *AccountRepository) Logout(ctx context.Context, id uint) error {
	return ar.DB.WithContext(ctx).Model(&models.Account{}).Where("id = ?", id).Updates(map[string]interface{}{"token": "", "refresh_token": ""}).Error
}

func (ar *AccountRepository) UpdateAvatar(ctx context.Context, accountID uint, avatarURL string) error {
	return ar.DB.WithContext(ctx).Model(&models.Account{}).Where("id = ?", accountID).Update("avatar_url", avatarURL).Error
}

func (ar *AccountRepository) UpdateToken(ctx context.Context, id uint, token string) error {
	return ar.DB.WithContext(ctx).Model(&models.Account{}).Where("id = ?", id).Update("token", token).Error
}

func (ar *AccountRepository) UpdateFields(ctx context.Context, id uint, updates map[string]interface{}) error {
	return ar.DB.WithContext(ctx).Model(&models.Account{}).Where("id = ?", id).Updates(updates).Error
}

func (ar *AccountRepository) FindAll(ctx context.Context) ([]*models.Account, error) {
	var accounts []*models.Account
	if err := ar.DB.WithContext(ctx).Find(&accounts).Error; err != nil {
		return nil, err
	}
	return accounts, nil
}
