// Package middleware i18n.go
package middleware

import (
	"github.com/kataras/iris/v12"
	"hot_keyword/config"
)

// SupportedLangs 是应用支持的语言列表, 将在 main.go 中动态填充
var SupportedLangs []string

func I18nMiddleware(ctx iris.Context) {
	defaultLang := config.Cfg.GetDefaultLang()
	var currentLang string

	// ==================== 最终且正确的逻辑 ====================
	// 我们已经知道 Iris 会把路径 /en-US/ 自动转为 URL 参数 ?lang=en-US。
	// 因此，检查这个参数是否存在，是判断“语言是否被用户在URL中显式指定”的最可靠方法。

	// 从 app.I18n 配置中获取当前使用的参数名，默认为 "lang"
	langParam := "lang"

	if ctx.URLParamExists(langParam) {
		// 如果 lang 参数存在，说明语言是用户通过 URL (路径或查询) 指定的。
		// 我们完全信任 Iris 的 locale 设置结果。
		currentLang = ctx.GetLocale().Language()
	} else {
		// 如果 lang 参数不存在，说明这是一个无前缀的 URL (如 /login)。
		// 此时 ctx.GetLocale() 的结果会受浏览器 Accept-Language 头影响。
		// 为了打破循环，我们必须强制使用默认语言。
		currentLang = defaultLang
		// 并且，关键一步：将我们的这个决定覆盖回 Context 中！
		ctx.SetLanguage(currentLang)
	}

	// 一个保险的 fallback
	if currentLang == "" {
		currentLang = defaultLang
	}

	// ==================== 后续的 ViewData 逻辑保持不变 ====================

	basePath := ctx.Path()

	langPrefix := ""
	if currentLang != defaultLang {
		langPrefix = "/" + currentLang
	}

	alternateURLs := make(map[string]string)
	for _, lang := range SupportedLangs {
		pathForURL := basePath
		if pathForURL == "" {
			pathForURL = "/"
		}

		if lang == defaultLang {
			alternateURLs[lang] = pathForURL
		} else {
			if pathForURL == "/" {
				alternateURLs[lang] = "/" + lang
			} else {
				alternateURLs[lang] = "/" + lang + pathForURL
			}
		}
	}

	ctx.ViewData("Lang", currentLang)
	ctx.ViewData("LangPrefix", langPrefix)
	ctx.ViewData("BasePath", basePath)
	ctx.ViewData("FullPath", langPrefix+basePath)
	ctx.ViewData("SupportedLangs", SupportedLangs)
	ctx.ViewData("DefaultLang", defaultLang)
	ctx.ViewData("AlternateURLs", alternateURLs)
	ctx.ViewData("Tr", ctx.Tr)

	ctx.Next()
}
