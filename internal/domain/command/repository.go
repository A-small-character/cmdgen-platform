package command

import "context"

// Repository 命令历史仓储接口
type Repository interface {
	Save(ctx context.Context, result *GenerateResult) error
	FindByID(ctx context.Context, id string) (*GenerateResult, error)
	FindByUserID(ctx context.Context, userID string, page, size int) ([]*GenerateResult, int64, error)
	Delete(ctx context.Context, id string) error
}

// Generator 命令生成器接口（领域服务）
type Generator interface {
	Generate(ctx context.Context, req *GenerateRequest) (*GenerateResult, error)
	SupportedCategory() Category
}
