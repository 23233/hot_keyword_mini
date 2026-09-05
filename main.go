// Package main main.go
package main

import (
	"github.com/23233/ggg/logger"
	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/middleware/cors"
	irisLogger "github.com/kataras/iris/v12/middleware/logger"
	irisRecover "github.com/kataras/iris/v12/middleware/recover"
	"github.com/kataras/realip"
	"hot_keyword/routers"
	"hot_keyword/routers/middleware"
	"io/fs"
	"strings"

	"hot_keyword/config"
	"hot_keyword/db"
	"hot_keyword/sdk"
	"hot_keyword/system"
	"os"
)

func main() {
	defer func() {
		if err := recover(); err != nil {
			logger.JM.Errorf("程序崩溃了 %v", err)
		}
	}()

	// 加载配置文件
	err := config.LoadConfig()
	if err != nil {
		panic(err)
	}
	// 初始化腾讯云 COS，未配置时仅禁用图片上传能力。
	sdk.InitCos()
	// 启动db
	err = db.InitDB(config.Cfg)
	if err != nil {
		panic(err)
	}
	// 连接redis
	err = db.InitRedis(config.Cfg)
	if err != nil {
		panic(err)
	}
	// 表结构合并
	err = system.Migrate()
	if err != nil {
		panic(err)
	}
	// 仅初始化管理员账户；业务数据必须由管理后台显式创建，查询接口不得隐式写入种子数据。
	if err = system.EnsureInitialAdmin(); err != nil {
		panic(err)
	}

	app := iris.New()
	app.Use(iris.Compression)
	app.Use(irisRecover.New())
	app.Use(irisLogger.New())

	// 跨域
	crs := cors.New()
	app.Use(crs.Handler())

	// i18n
	var localeFS fs.FS
	localeFS = os.DirFS("locales")
	err = app.I18n.LoadFS(localeFS, "*.ini")
	if err != nil {
		logger.JM.ErrorE(err, "i18n load failed")
		return
	}

	// 动态填充支持的语言列表
	files, err := fs.ReadDir(localeFS, ".")
	if err != nil {
		logger.JM.ErrorE(err, "无法读取 locales 目录")
		return
	}
	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".ini") {
			lang := strings.TrimSuffix(file.Name(), ".ini")
			middleware.SupportedLangs = append(middleware.SupportedLangs, lang)
		}
	}
	logger.JM.Infof("检测到支持的语言: %v", middleware.SupportedLangs)

	app.I18n.SetDefault(config.Cfg.GetDefaultLang())
	logger.JM.Infof("默认语言: %s", config.Cfg.GetDefaultLang())

	app.HandleDir("/static", "./static")
	tmpl := iris.Django("templates", ".html").Reload(true)

	app.RegisterView(tmpl)

	if !config.Pro {
		app.Logger().SetLevel("debug")
	} else {
		app.Logger().SetLevel("warn")
	}

	app.Get("/ip", func(ctx iris.Context) {
		ip := realip.Get(ctx.Request())
		_, _ = ctx.WriteString(ip)
	})

	// 系统运行日志端点：严禁生产环境无鉴权公开，仅在非生产环境或经管理员认证后方可查看
	if !config.Pro && !config.Cfg.IsProduction() {
		debugParty := app.Party("/debug", middleware.AdminAuthMiddleware)
		{
			debugParty.Get("/log", iris.FromStd(logger.JM.ViewQueueFunc))
			debugParty.Get("/log_stats", iris.FromStd(logger.JM.ViewStatsFunc))
		}
	}

	routers.RegisterRouters(app)

	app.HandleDir("/assets", iris.Dir("./assets"))

	err = app.Run(iris.Addr(":8080"), iris.WithoutServerError(iris.ErrServerClosed), iris.WithoutBodyConsumptionOnUnmarshal, iris.WithOptimizations)
	if err != nil {
		logger.JM.ErrorE(err, "服务器启动失败")
	}
}
