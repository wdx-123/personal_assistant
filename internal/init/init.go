package init

import (
	"context"
	"os"
	"time"

	"personal_assistant/flag"
	"personal_assistant/global"
	"personal_assistant/internal/controller"
	apiSystem "personal_assistant/internal/controller/system"
	"personal_assistant/internal/core"
	"personal_assistant/internal/infrastructure"
	"personal_assistant/internal/repository"
	"personal_assistant/internal/repository/adapter"

	"github.com/joho/godotenv"

	"go.uber.org/zap"

	"personal_assistant/internal/service"
	"personal_assistant/internal/service/system"
)

func Init() {
	// 尝试加载 .env 文件，
	// 如果不存在也不报错（生产环境可能直接用环境变量）
	_ = godotenv.Load()
	// 初始化配置
	core.InitConfig("configs")
	// 初始化日志
	global.Log = core.InitLogger()
	infrastructure.Init()
	// 为jwt黑名单开启本地存储
	core.OtherInit()
	// 连接数据库，初始化gorm
	global.DB = core.InitGorm()
	// 开启自动迁移
	if global.Config.System.AutoMigrate {
		if err := flag.SQL(); err != nil {
			global.Log.Error("auto migrate failed", zap.Error(err))
			os.Exit(1)
		}
	}
	// 连接redis
	global.Redis = core.ConnectRedis()
	// 初始化Casbin
	core.InitCasbin()
	// 初始化存储驱动（本地/七牛，七牛自动包装熔断器）
	core.InitStorage()
	// 初始化上传限流器（依赖 Redis）
	core.InitUploadRateLimiters()
	// 初始化flag
	flag.InitFlag()
	// 初始化Repository层
	mysqlAdapter := &adapter.MySQLAdapter{}
	mysqlAdapter.SetConnection(global.DB) // 使用现有的数据库连接
	repository.InitRepositoryGroupWithAdapter(mysqlAdapter)
	// 题库预热，luogu题库所有题目进行存储
	core.StartLuoguQuestionBankWarmup(
		context.Background(),
		repository.GroupApp.SystemRepositorySupplier.GetLuoguQuestionBankRepository(),
	)
	// 题库预热，LeetCode题库所有题目进行存储
	core.StartLeetcodeQuestionBankWarmup(
		context.Background(),
		repository.GroupApp.SystemRepositorySupplier.GetLeetcodeQuestionBankRepository(),
	)

	// 开启Outbox Relay,进行时间传递
	core.StartOutboxRelay(
		context.Background(),
		repository.GroupApp.SystemRepositorySupplier.GetOutboxRepository(),
		global.Redis,
		global.Log)

	// 加载jwt黑名单（使用Repository层）
	// 为初始化操作设置30秒超时，避免启动时卡死
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	system.LoadAllWithRepository(ctx, repository.GroupApp)

	// 业务函数--单例
	service.GroupApp = &service.Group{
		SystemServiceSupplier: system.SetUp(repository.GroupApp),
	}
	if err := core.InitSubscribers(
		context.Background(),
		service.GroupApp.SystemServiceSupplier.GetOJSvc()); err != nil {
		global.Log.Error("init subscribers failed", zap.Error(err))
	}

	// 控制函数
	controller.ApiGroupApp = &controller.ApiGroup{
		SystemApiGroup: apiSystem.SetUp(service.GroupApp),
	}
	// 同步权限数据
	permissionService := service.GroupApp.SystemServiceSupplier.GetPermissionSvc()
	if err := permissionService.SyncAllPermissionsToCasbin(ctx); err != nil {
		global.Log.Error("权限同步失败", zap.Error(err))
	}
	// 启动定时任务
	core.InitCron()

	// 开启函数
	core.RunServer()
}
