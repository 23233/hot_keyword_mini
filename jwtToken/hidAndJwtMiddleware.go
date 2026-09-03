package jwtToken

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v4"
	irisJwt "github.com/iris-contrib/middleware/jwt"
	"github.com/kataras/iris/v12"
)

var jwtParser = new(jwt.Parser)

// token 验证
func tokenCheck(token string, m *irisJwt.Middleware, ctx iris.Context) (error, *jwt.Token) {

	parsedToken, err := jwtParser.Parse(token, m.Config.ValidationKeyGetter)
	if err != nil {
		return err, parsedToken
	}

	if m.Config.SigningMethod != nil && m.Config.SigningMethod.Alg() != parsedToken.Header["alg"] {
		err := fmt.Errorf("Expected %s signing method but token specified %s",
			m.Config.SigningMethod.Alg(),
			parsedToken.Header["alg"])
		return err, parsedToken
	}

	// Check if the parsed token is valid...
	if !parsedToken.Valid {
		m.Config.ErrorHandler(ctx, irisJwt.ErrTokenInvalid)
		return irisJwt.ErrTokenInvalid, parsedToken
	}

	if m.Config.Expiration {
		if claims, ok := parsedToken.Claims.(jwt.MapClaims); ok {
			if expired := claims.VerifyExpiresAt(time.Now().Unix(), true); !expired {
				return irisJwt.ErrTokenExpired, parsedToken
			}
		}
	}
	ctx.Values().Set(m.Config.ContextKey, parsedToken)
	return nil, parsedToken
}

// SidAndJwtTokenCheck params上的c_t或header jwt双验证
func SidAndJwtTokenCheck(ctx iris.Context) (error, string) {
	err := CustomJwt.CheckJWT(ctx)
	if err != nil {
		return err, "header"
	}
	return nil, ""
}

// SidAndJwtMiddleware params上的sid或header jwt双验证
func SidAndJwtMiddleware(ctx iris.Context) {
	err, from := SidAndJwtTokenCheck(ctx)
	if err != nil {
		if from == "c_t" {
			CtJwt.Config.ErrorHandler(ctx, err)
			return
		}
		CustomJwt.Config.ErrorHandler(ctx, err)
		return
	}
	ctx.Next()
}

func SidAndJwtHasMiddleware(ctx iris.Context) {
	_, _ = SidAndJwtTokenCheck(ctx)
	ctx.Next()
}
