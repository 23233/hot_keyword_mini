package jwtToken

import "time"

var (
	JwtUserModel  = "userModel"
	JwtUserId     = "userId"
	JwtUserOpenId = "userOpenId"
	JwtExpired    = time.Hour * 24 * 30
)
