package db

import (
	"context"
	"gorm_template/config"

	"github.com/23233/ggg/logger"
	"github.com/redis/go-redis/v9"
)

var Redis *redis.Client

func InitRedis(cfg *config.Config) error {
	if cfg.RedisEnable == false {
		logger.JM.Infof("redis 配置未启用")
		return nil
	}
	addr := cfg.RedisHost + ":" + cfg.RedisPort
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})

	// 确保 Redis 连接正常
	_, err := rdb.Ping(context.Background()).Result()
	if err != nil {
		logger.JM.ErrorE(err, " Redis connection error")
		return err
	}
	logger.JM.Infof("redis 连接成功")
	Redis = rdb
	return err
}
