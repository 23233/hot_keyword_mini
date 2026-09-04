// Package routers admin_user.go
package routers

import (
	"hot_keyword/routers/middleware"
	"hot_keyword/services"
	"strconv"

	"github.com/kataras/iris/v12"
)

// AdminLoginReq 管理员登录入参
type AdminLoginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// CreateAdminUserReq 创建管理员入参
type CreateAdminUserReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
	RealName string `json:"real_name"`
	Role     string `json:"role"`
}

// UpdateAdminUserReq 更新管理员入参
type UpdateAdminUserReq struct {
	RealName    string `json:"real_name"`
	Role        string `json:"role"`
	Status      string `json:"status"`
	NewPassword string `json:"new_password"`
}

// RegisterAdminUserRoutes 注册管理员用户管理及认证相关路由
func RegisterAdminUserRoutes(adminParty iris.Party) {
	authService := services.NewAdminAuthService()

	// 1. 管理员登录
	adminParty.Post("/auth/login", func(ctx iris.Context) {
		var req AdminLoginReq
		if err := ctx.ReadJSON(&req); err != nil {
			ctx.JSON(iris.Map{"code": 400, "msg": "请求参数无效"})
			return
		}

		user, token, err := authService.AdminLogin(req.Username, req.Password)
		if err != nil {
			ctx.JSON(iris.Map{"code": 401, "msg": err.Error()})
			return
		}

		// 写入 Cookie 便于浏览器会话维护
		ctx.SetCookieKV("admin_token", token, iris.CookieExpires(24*60*60))

		ctx.JSON(iris.Map{
			"code": 0,
			"msg":  "登录成功",
			"data": iris.Map{
				"token": token,
				"user":  user,
			},
		})
	})

	// 2. 获取当前登录者身份
	adminParty.Get("/auth/me", func(ctx iris.Context) {
		claims := ctx.Values().Get("admin_claims")
		ctx.JSON(iris.Map{
			"code": 0,
			"msg":  "success",
			"data": claims,
		})
	})

	// 3. 管理员登出
	adminParty.Post("/auth/logout", func(ctx iris.Context) {
		ctx.RemoveCookie("admin_token")
		ctx.JSON(iris.Map{
			"code": 0,
			"msg":  "已成功退出登录",
		})
	})

	// 4. 管理员用户列表 (List)
	adminParty.Get("/users", func(ctx iris.Context) {
		list, err := authService.ListAdminUsers()
		if err != nil {
			ctx.JSON(iris.Map{"code": 500, "msg": "获取管理员列表失败: " + err.Error()})
			return
		}
		ctx.JSON(iris.Map{
			"code": 0,
			"msg":  "success",
			"data": list,
		})
	})

	// 5. 新建管理员 (Create) - 仅限超级管理员
	adminParty.Post("/users", middleware.RequireAdminRole("super_admin"), func(ctx iris.Context) {
		var req CreateAdminUserReq
		if err := ctx.ReadJSON(&req); err != nil {
			ctx.JSON(iris.Map{"code": 400, "msg": "参数反序列化失败"})
			return
		}

		newUser, err := authService.CreateAdminUser(req.Username, req.Password, req.RealName, req.Role)
		if err != nil {
			ctx.JSON(iris.Map{"code": 400, "msg": err.Error()})
			return
		}

		ctx.JSON(iris.Map{
			"code": 0,
			"msg":  "管理员创建成功",
			"data": newUser,
		})
	})

	// 6. 修改管理员信息或密码 (Update) - 仅限超级管理员
	adminParty.Put("/users/{id:int64}", middleware.RequireAdminRole("super_admin"), func(ctx iris.Context) {
		id, _ := ctx.Params().GetInt64("id")
		var req UpdateAdminUserReq
		if err := ctx.ReadJSON(&req); err != nil {
			ctx.JSON(iris.Map{"code": 400, "msg": "参数无效"})
			return
		}

		updatedUser, err := authService.UpdateAdminUser(id, req.RealName, req.Role, req.Status, req.NewPassword)
		if err != nil {
			ctx.JSON(iris.Map{"code": 400, "msg": err.Error()})
			return
		}

		ctx.JSON(iris.Map{
			"code": 0,
			"msg":  "管理员信息更新成功",
			"data": updatedUser,
		})
	})

	// 7. 删除管理员 (Delete) - 仅限超级管理员
	adminParty.Delete("/users/{id:int64}", middleware.RequireAdminRole("super_admin"), func(ctx iris.Context) {
		id, _ := ctx.Params().GetInt64("id")
		var currentAdminID int64
		if rawID := ctx.Values().Get("admin_id"); rawID != nil {
			if num, ok := rawID.(int64); ok {
				currentAdminID = num
			} else if strID, ok := rawID.(string); ok {
				currentAdminID, _ = strconv.ParseInt(strID, 10, 64)
			}
		}

		if err := authService.DeleteAdminUser(id, currentAdminID); err != nil {
			ctx.JSON(iris.Map{"code": 400, "msg": err.Error()})
			return
		}

		ctx.JSON(iris.Map{
			"code": 0,
			"msg":  "管理员已成功删除",
		})
	})
}
