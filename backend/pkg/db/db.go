package db

import (
	"feedsystem_ai_go/internal/models"
	"feedsystem_ai_go/internal/worker"
	"feedsystem_ai_go/pkg/config"
	"fmt"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func NewDB(dbcfg config.DatabaseConfig) (*gorm.DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		dbcfg.User, dbcfg.Password, dbcfg.Host, dbcfg.Port, dbcfg.DBName)

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	return db, nil
}

func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&models.Account{}, &models.Video{}, &models.Like{}, &models.Comment{},
		&models.Social{}, &models.OutboxMsg{}, &models.Tag{}, &models.VideoTag{},
		&models.Message{}, &worker.Notification{},
		&models.MediaFileRecord{},
	)
}

func CloseDB(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
