package models

import (
	"time"
)

// User 表示一个用户账户信息
type User struct {
	ID                  int64      `gorm:"column:id;primaryKey;autoIncrement;comment:ID" json:"id,omitempty"`
	CreatedAt           time.Time  `gorm:"column:created_at;comment:创建时间" json:"created_at"`
	UpdatedAt           time.Time  `gorm:"column:updated_at;comment:更新时间" json:"updated_at"`
	PhoneNumber         string     `gorm:"column:phone_number;index;comment:手机号" json:"phone_number,omitempty"`
	WechatOpenID        string     `gorm:"column:wechat_openid;index;comment:微信OpenID" json:"wechat_open_id,omitempty"`
	WechatUnionID       string     `gorm:"column:wechat_unionid;index;comment:微信UnionID" json:"wechat_union_id,omitempty"`
	AvatarUrl           string     `gorm:"column:avatar_url;comment:用户头像" json:"avatar_url,omitempty"`   // 系统头像为名称
	AvatarType          int        `gorm:"column:avatar_type;comment:头像类型" json:"avatar_type,omitempty"` // 0 系统头像 1 微信头像 2 自己上传的头像
	Gold                int64      `gorm:"column:gold;comment:用户总积分" json:"gold,omitempty"`              //
	Diamond             int64      `gorm:"column:diamond;comment:用户钻石" json:"diamond,omitempty"`
	NickName            string     `gorm:"column:nickname;comment:用户昵称" json:"nick_name,omitempty"`
	Sex                 bool       `gorm:"column:sex;comment:性别" json:"sex,omitempty"`                             // true是男 1是男 0是女
	RealVerity          bool       `gorm:"column:real_verity;comment:是否实名认证" json:"real_verity,omitempty"`         // 是否实名认证
	IsOnline            bool       `gorm:"column:is_online;default:false;comment:是否在线" json:"is_online,omitempty"` //
	WechatAccessToken   *string    `gorm:"column:wechat_access_token;comment:微信access_token" json:"wechat_access_token,omitempty"`
	WechatRefreshToken  *string    `gorm:"column:wechat_refresh_token;comment:微信refresh_token" json:"wechat_refresh_token,omitempty"`
	WechatTokenGetAt    *time.Time `gorm:"column:wechat_token_get_at;comment:微信token获取时间" json:"wechat_token_get_at,omitempty"`
	WechatTokenExpireAt *time.Time `gorm:"column:wechat_token_expire_at;comment:微信token过期时间" json:"wechat_token_expire_at,omitempty"`
	Language            string     `gorm:"column:language;default:'zh-CN';comment:用户语言" json:"language,omitempty"`
	Username            string     `gorm:"column:username;index;comment:用户名" json:"username,omitempty"`
	PasswordHash        string     `gorm:"column:password_hash;comment:密码哈希" json:"password_hash,omitempty"`
}

// TableName 自定义User模型的表名
func (u *User) TableName() string {
	return "users"
}

func (u *User) SimpleUser() *User {
	return &User{
		ID:           u.ID,
		NickName:     u.NickName,
		AvatarUrl:    u.AvatarUrl,
		AvatarType:   u.AvatarType,
		Sex:          u.Sex,
		Gold:         u.Gold,
		Diamond:      u.Diamond,
		Language:     u.Language,
		Username:     u.Username,
		WechatOpenID: u.WechatOpenID,
	}
}
