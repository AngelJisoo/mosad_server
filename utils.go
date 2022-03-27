package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"golang.org/x/crypto/bcrypt"
)
//
func CheckDto(c *gin.Context, dto interface{}) bool {
	// gin.Context.Bind( type struct ) 支持所有的类型的自动解析
	err := c.Bind(dto)
	if err != nil {
		MakeErrorResponse(c, 400, "请求参数不合规范: %s", err)
		return false
	}
	// 使用Validator进行传入的参数检验
	err = Validator.Struct(dto)
	if err != nil {
		MakeErrorResponse(c, 400, "请求参数不对: %s", err)
		return false
	}
	return true
}

func MustGetUser(c *gin.Context) *User {
	userInterface, ok := c.Get(UserIdentityKey)
	if !ok {
		panic(fmt.Errorf("MustGetUser c.Get() failed"))
	}
	user, ok := userInterface.(*User)
	if !ok {
		panic(fmt.Errorf("MustGetUser interface conversion failed"))
	}
	return user
}
//保存用户信息
func SaveUser(c *gin.Context, user *User) bool {
	//将json格式的struct解析为string
	marshalled, err := json.Marshal(user)
	if err != nil {
		log.Default().Printf("failed to marshal signed up user data: %s", err)
		MakeErrorResponse(c, 500, "系统内部出错，注册失败")
		return false
	}
	//将marshalled保存于HASH_USER_DATA哈希表中
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	err = RedisClient.HSet(ctx, HASH_USER_DATA, user.Username, string(marshalled)).Err()
	cancel()
	if err != nil {
		log.Default().Printf("failed to save signed up user data to redis: %s", err)
		MakeErrorResponse(c, 500, "系统内部出错，注册失败")
		return false
	}
	return true
}
// 保存用户密码的hash值，传入password和username，在对密码进行hash加密后设置
func SavePassword(c *gin.Context, password string, username string) bool {
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Default().Printf("failed to call bcrypt: %s", err)
		MakeErrorResponse(c, 500, "系统内部出错，注册失败")
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	//将得到的hash后的密码进行设置
	err = RedisClient.HSet(ctx, HASH_USER_PASSWORDS, username, string(hashed)).Err()
	cancel()
	if err != nil {
		log.Default().Printf("failed to save signed up user's password to redis: %s", err)
		MakeErrorResponse(c, 500, "系统内部出错，注册失败")
		return false
	}
	return true
}

type Response struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}
//返回响应错误
func MakeErrorResponse(c *gin.Context, code int, message string, args ...interface{}) {
	data, err := json.Marshal(Response{Code: code, Message: fmt.Sprintf(message, args...)})
	if err != nil {
		//系统出错，返回code=500
		log.Default().Printf("failed to marshal response: %s", err)
		c.Data(500, gin.MIMEJSON, []byte(`{"code":500,"message":"系统内部出错"}`))
		return
	}
	c.Data(code, gin.MIMEJSON, data)
}
//响应，args暂时没用到
func MakeResponse(c *gin.Context, code int, message string, args ...interface{}) {
	data, err := json.Marshal(Response{Code: code, Message: fmt.Sprintf(message, args...)})
	if err != nil {
		//系统出错，返回code=500
		log.Default().Printf("failed to marshal response: %s", err)
		c.Data(500, gin.MIMEJSON, []byte(`{"code":500,"message":"系统内部出错"}`))
		return
	}
	c.Data(code, gin.MIMEJSON, data)
}

var Validator = validator.New()

func FileExists(path string) bool {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func DirectoryExists(path string) bool {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return info.IsDir()
}

func CreateDirectoryIfNotExists(path string) error {
	//先要判断是否存在同名目录和同名文件
	if FileExists(path) {
		return fmt.Errorf("cannot create directory %s because a file with the same name already exists", path)
	}
	if !DirectoryExists(path) {
		//创建多级目录，0700是只有拥有者有读、写、执行权限。
		err := os.MkdirAll(path, 0700)
		if err != nil {
			return err
		}
	}
	return nil
}
//将URL字符串进行连接
func JoinUrl(base string, paths ...string) string {
	//使用Path包的 Join函数将URL字符串进行连接成一个路径
	p := path.Join(paths...)
	//连接
	return fmt.Sprintf("%s/%s", strings.TrimRight(base, "/"), strings.TrimLeft(p, "/"))
}
