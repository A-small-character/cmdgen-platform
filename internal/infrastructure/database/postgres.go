package database

import (
	"fmt"
	"time"

	"github.com/cmdgen/platform/pkg/config"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func NewPostgres(cfg *config.DatabaseConfig) (*gorm.DB, error) {
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s TimeZone=Asia/Shanghai",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Name, cfg.SSLMode)

	gormCfg := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	}

	db, err := gorm.Open(postgres.Open(dsn), gormCfg)
	if err != nil {
		return nil, fmt.Errorf("连接PostgreSQL失败: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	return db, nil
}

// Migrate 自动迁移数据库
func Migrate(db *gorm.DB, models ...interface{}) error {
	return db.AutoMigrate(models...)
}

// CommandHistoryModel 命令历史数据模型
type CommandHistoryModel struct {
	ID          string    `gorm:"primaryKey;type:uuid"`
	UserID      string    `gorm:"index;type:uuid"`
	RequestID   string    `gorm:"index;type:uuid"`
	Category    string    `gorm:"index"`
	UserInput   string    `gorm:"type:text"`
	ResultJSON  string    `gorm:"type:jsonb"`
	AIProvider  string
	TokensUsed  int
	LatencyMS   int64
	CreatedAt   time.Time `gorm:"index"`
	UpdatedAt   time.Time
}

func (CommandHistoryModel) TableName() string { return "command_histories" }

// KnowledgeItemModel 知识条目数据模型
type KnowledgeItemModel struct {
	ID        string    `gorm:"primaryKey;type:uuid"`
	Title     string    `gorm:"index"`
	Content   string    `gorm:"type:text"`
	Type      string    `gorm:"index"`
	Tags      string    `gorm:"type:text"`
	Source    string    `gorm:"index"`
	SourceURL string
	Version   string
	Vendor    string    `gorm:"index"`
	CreatedAt time.Time
	UpdatedAt time.Time `gorm:"index"`
}

func (KnowledgeItemModel) TableName() string { return "knowledge_items" }

// UserModel 用户数据模型
type UserModel struct {
	ID        string    `gorm:"primaryKey;type:uuid"`
	Username  string    `gorm:"uniqueIndex"`
	Email     string    `gorm:"uniqueIndex"`
	Password  string
	Role      string    `gorm:"default:user"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (UserModel) TableName() string { return "users" }
