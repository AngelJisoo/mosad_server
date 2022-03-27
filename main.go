package main

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"time"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/spf13/viper"
)

const (
	ImagesDirectory = "images"
)

var (
	ServiceUrl  string
	RedisClient *redis.Client
)

func main() {
	viper.SetConfigName("config")// 配置文件名 (无后缀)
	viper.SetConfigType("toml")//规定配置文件格式为toml
	viper.AddConfigPath(".")// 配置文件所在路径
	viper.AddConfigPath("dev")// 添加多个路径

	err := viper.ReadInConfig()//寻找并读取配置文件
	if err != nil {
		panic(fmt.Errorf("Fatal error config file: %s \n", err))//panic宕机并报错
	}
	//监控配置文件的变化
	// viper.WatchConfig()
	// viper.OnConfigChange(func(e fsnotify.Event) {
	// 	fmt.Println("Config file changed:", e.Name)
	// })
	
	//获取配置文件的字符串值
	ServiceUrl = viper.GetString("url")
	//初始化redis
	redisClient := redis.NewClient(&redis.Options{
		Addr:     viper.GetString("redis.address"),
		Password: viper.GetString("redis.password"),
		DB:       viper.GetInt("redis.database"),
	})
	log.Default().Printf("connecting to redis: %s", redisClient.Options().Addr)
    //生成一个可取消的ctx，返回ctx和它的取消函数cancel，使用结束后调用cancel()
	//测试连接redis
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	err = redisClient.Ping(ctx).Err()
	cancel()
	if err != nil {
		panic(fmt.Errorf("failed to connect to redis: %w", err))
	}
	RedisClient = redisClient
	
	err = CreateDirectoryIfNotExists(ImagesDirectory)
	if err != nil {
		panic(fmt.Errorf("failed to create images directory: %w", err))
	}
	log.Default().Printf("launching gin")
	//初始化WEB服务端gin
	r := gin.New()
	r.Use(gin.Logger())
	r.Use(gin.Recovery())

	//初始化鉴权中间件（在auth.go中定义）
	authMiddleware, err := jwt.New(JwtAuthMiddleware)
	if err != nil {
		panic(fmt.Errorf("failed to initialize auth middleware: %w", err))
	}
	//静态加载图片
	r.Static("/images", filepath.Join(".", ImagesDirectory))
	//登录登出注册
	r.POST("/login", authMiddleware.LoginHandler)
	r.POST("/logout", authMiddleware.LogoutHandler)
	r.POST("/signup", SignUpHandler)
	//启用中间件
	r.Use(authMiddleware.MiddlewareFunc())
	r.Use(BlockUnauthorizedMiddleware)
	r.Use()
	//定义于handlers.go，处理数据请求
	r.GET("/getmes", GetUserDataHandler)
	r.GET("/getlist", GetListHandler)
	r.POST("/resetpoints", ResetPointsHandler)
	r.POST("/add", AddRecordHandler)
	r.DELETE("/delete", DeleteRecordHandler)
	r.PUT("/modify", ModifyUserDataHandler)
	// 启动服务端，于address
	r.Run(viper.GetString("address"))
}
