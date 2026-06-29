package bootstrap

import (
	"fmt"

	"secmgmt_go/internal/config"
	"secmgmt_go/internal/database"
	"secmgmt_go/internal/http/handler"
	"secmgmt_go/internal/http/router"
	"secmgmt_go/internal/repository"
	"secmgmt_go/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type App struct {
	Config *config.Config
	Logger *zap.Logger
	Router *gin.Engine
	Close  func()
}

func Build(rootDir string) (*App, error) {
	cfg, err := config.Load(rootDir)
	if err != nil {
		return nil, err
	}

	logger, err := zap.NewProduction()
	if err != nil {
		return nil, fmt.Errorf("init logger: %w", err)
	}

	db, err := database.New(cfg.MySQLDSN)
	if err != nil {
		return nil, err
	}
	if err := database.Migrate(db); err != nil {
		return nil, err
	}

	repo := repository.New(db)
	authService := service.NewAuthService(cfg, repo)
	queryService := service.NewQueryService(repo)
	platformService := service.NewPlatformService(cfg, repo, logger)
	operationLogService := service.NewOperationLogService(repo, logger)
	if err := operationLogService.EnsureBootstrapData(); err != nil {
		return nil, err
	}
	stopOperationLogCleanup := operationLogService.StartCleanupJob()
	hikvisionBridgeService := service.NewHikvisionAlarmBridgeService(cfg, repo, logger)
	platformService.SetHikvisionAlarmBridge(hikvisionBridgeService)
	if err := hikvisionBridgeService.Start(); err != nil {
		logger.Warn("start hikvision alarm bridge", zap.Error(err))
	}

	engine := router.New(cfg, repo, operationLogService, router.Handlers{
		Auth:      handler.NewAuthHandler(authService),
		Query:     handler.NewQueryHandler(queryService),
		Platform:  handler.NewPlatformHandler(cfg, authService, queryService, platformService),
		Operation: handler.NewOperationLogHandler(operationLogService),
	})

	return &App{
		Config: cfg,
		Logger: logger,
		Router: engine,
		Close: func() {
			stopOperationLogCleanup()
			hikvisionBridgeService.Stop()
		},
	}, nil
}
