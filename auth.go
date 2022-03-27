package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"golang.org/x/crypto/bcrypt"
)

const UserIdentityKey = "user"

type LoginPayload struct {
	Username string
	Password string
}

type User struct {
	Username string `json:"username"`
	Nickname string `json:"nickname"`
	Points   int    `json:"points"`
	Phone    string `json:"phone"`
	ImageUrl string `json:"imgUrl"`
}
// https://github.com/appleboy/gin-jwt demo
var JwtAuthMiddleware = &jwt.GinJWTMiddleware{
	Realm:       "mosad-server",
	Key:         []byte("auw89daph28dh2a98d720gf17gt67812tge78621t86721"),
	Timeout:     time.Hour * 6,
	MaxRefresh:  time.Hour * 6,
	IdentityKey: UserIdentityKey,
	IdentityHandler: func(c *gin.Context) interface{} {
		claims := jwt.ExtractClaims(c)
		username, ok := claims[UserIdentityKey]
		if !ok {
			return nil
		}

		usernameString, ok := username.(string)
		if !ok {
			return nil
		}
		//用户信息获取
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		data, err := RedisClient.HGet(ctx, HASH_USER_DATA, usernameString).Result()
		cancel()
		if err == redis.Nil {
			return nil
		}
		if err != nil {
			log.Default().Printf("failed to get signed up user data from redis: %s", err)
			return nil
		}
		var user User
		err = json.Unmarshal([]byte(data), &user)
		if err != nil {
			log.Default().Printf("failed to unmarshal user data: %s", err)
			return nil
		}

		return &user
	},
	Authenticator: func(c *gin.Context) (interface{}, error) {
		var payload LoginPayload
		if err := c.ShouldBind(&payload); err != nil {
			return "", jwt.ErrMissingLoginValues
		}
		//用户密码验证
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		realPassword, err := RedisClient.HGet(ctx, HASH_USER_PASSWORDS, payload.Username).Result()
		cancel()
		if err == redis.Nil {
			return nil, fmt.Errorf("用户名或密码错误")
		}
		if err != nil {
			log.Default().Fatalf("failed to read user password from redis: %s", err)
			return nil, fmt.Errorf("系统内部出错，验证失败")
		}

		if err := bcrypt.CompareHashAndPassword([]byte(realPassword), []byte(payload.Password)); err != nil {
			return nil, fmt.Errorf("用户名或密码错误")
		}

		ctx, cancel = context.WithTimeout(context.Background(), time.Second*5)
		data, err := RedisClient.HGet(ctx, HASH_USER_DATA, payload.Username).Result()
		cancel()
		if err != nil {
			log.Default().Fatalf("failed to read user data from redis: %s", err)
			return nil, fmt.Errorf("系统内部出错，验证失败")
		}

		var user User
		err = json.Unmarshal([]byte(data), &user)
		if err != nil {
			log.Default().Fatalf("failed to unmarshal user data: %s", err)
			return nil, fmt.Errorf("系统内部出错，验证失败")
		}
		return user.Username, nil
	},
	//User存在则返回map[string]interface{} {UserIdentityKey: username}
	//否则返回空的map
	//jwt.MapClaims 定义为 map[string]interface{}
	PayloadFunc: func(id interface{}) jwt.MapClaims {
		if username, ok := id.(string); ok {
			return jwt.MapClaims{UserIdentityKey: username}
		}
		panic(fmt.Errorf("jwt PayloadFunc failed"))
	},
	TokenLookup:   "header: Authorization, query: token, cookie: jwt",
	TokenHeadName: "Bearer",
	TimeFunc:      time.Now,
}

var BlockUnauthorizedMiddleware = func(c *gin.Context) {
	user, ok := c.Get(UserIdentityKey)
	if !ok {
		MakeErrorResponse(c, http.StatusUnauthorized, "未登录")
	}
	_, ok = user.(*User)
	if !ok {
		MakeErrorResponse(c, http.StatusUnauthorized, "未登录")
	}
}
