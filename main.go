package main

import (
	"github.com/23233/ggg/logger"
	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/middleware/cors"
	irisLogger "github.com/kataras/iris/v12/middleware/logger"
	irisRecover "github.com/kataras/iris/v12/middleware/recover"
	"github.com/kataras/realip"
	"gorm_template/routers"
	"gorm_template/routers/middleware"
	"io/fs"
	"strings"

	"gorm_template/config"
	"gorm_template/db"
	"gorm_template/system"
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
	app.Get("/debug/log", iris.FromStd(logger.JM.ViewQueueFunc))
	app.Get("/debug/log_stats", iris.FromStd(logger.JM.ViewStatsFunc))

	routers.RegisterRouters(app)

	app.HandleDir("/assets", iris.Dir("./assets"))

	err = app.Run(iris.Addr(":8080"), iris.WithoutServerError(iris.ErrServerClosed), iris.WithoutBodyConsumptionOnUnmarshal, iris.WithOptimizations)
	if err != nil {
		logger.JM.ErrorE(err, "服务器启动失败")
	}
}
