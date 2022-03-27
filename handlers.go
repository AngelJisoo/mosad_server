package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	gonanoid "github.com/matoous/go-nanoid/v2"
)
// Data Transfer Object定义
// 每个成员后面的标签：1、所用的格式和在格式中的名字，如form:"username"，json:"type"
// 2、用于参数校验，如validate:"required"
type SignupDto struct {
	Username  string                `form:"username" validate:"required"`
	Nickname  string                `form:"nickname" validate:"required"`
	Phone     string                `form:"phone" validate:"required"`
	Password  string                `form:"password" validate:"required"`
	ImageData *multipart.FileHeader `form:"img" validate:"required"`
}
//用户注册
func SignUpHandler(c *gin.Context) {
	var dto SignupDto
	//由于在CheckDto中会调用c.Bind(dto),所以这里必须传入dto的引用
	if !CheckDto(c, &dto) {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	res, err := RedisClient.HGet(ctx, HASH_USER_PASSWORDS, dto.Username).Result()
	//在HASH_USER_PASSWORDS哈希表中，HGet key值为dto.Username，查看是否已存在用户名
	cancel()
	if res != "" {
		MakeErrorResponse(c, 400, "此用户名已被注册")
		return
	}
	if err != redis.Nil {
		log.Default().Printf("failed to call redis: %s", err)
		MakeErrorResponse(c, 500, "系统内部出错，注册失败")
		return
	}

	if !SavePassword(c, dto.Password, dto.Username) {
		//保存密码，如果出错则return，目前尚未异常处理
		return
	}
	//处理头像
	//Ext：返回文件的扩展名
	ext := filepath.Ext(dto.ImageData.Filename)
	//生成唯一ID文件名
	fileName := fmt.Sprintf("%s%s", gonanoid.Must(), ext)
	//c.SaveUploadedFile保存文件到服务器，路径是 Image目录/filename图片唯一文件名
	if err := c.SaveUploadedFile(dto.ImageData, filepath.Join(ImagesDirectory, fileName)); err != nil {
		log.Default().Printf("failed to save image %s: %s", fileName, err)
		MakeErrorResponse(c, 500, "系统内部出错，注册失败")
		return
	}
	//图像的URL即服务器的URL的images目录下的filename
	user := &User{
		Username: dto.Username,
		Nickname: dto.Nickname,
		Points:   0,
		Phone:    dto.Phone,
		ImageUrl: JoinUrl(ServiceUrl, "images", fileName),
	}
	//保存用户信息到数据库
	if !SaveUser(c, user) {
		return
	}

	MakeResponse(c, 200, "注册成功")
}

func GetUserDataHandler(c *gin.Context) {
	user, _ := c.Get(UserIdentityKey)
	c.JSON(http.StatusOK, user)
}

type ModifyUserDataDto struct {
	Username  string                `form:"username"`
	Nickname  string                `form:"nickname"`
	Phone     string                `form:"phone"`
	Password  string                `form:"password"`
	ImageData *multipart.FileHeader `form:"img"`
}

//修改用户信息
func ModifyUserDataHandler(c *gin.Context) {
	var dto ModifyUserDataDto
	if !CheckDto(c, &dto) {
		return
	}
	//获取当前用户资料
	user := MustGetUser(c)

	if dto.Username != "" && user.Username != dto.Username {
		MakeErrorResponse(c, 400, "不可以修改账号名")
		return
	}

	if user.Nickname != dto.Nickname {
		user.Nickname = dto.Nickname
	}

	if user.Phone != dto.Phone {
		user.Phone = dto.Phone
	}

	if dto.ImageData != nil {
		//于服务器调用 os包 的Remove进行删除原文件
		//filepath.Base用于将文件名从路径名中裁出
		_ = os.Remove(filepath.Join(ImagesDirectory, filepath.Base(user.ImageUrl)))
		//类似注册时的操作，生成新的图片
		ext := filepath.Ext(dto.ImageData.Filename)
		fileName := fmt.Sprintf("%s%s", gonanoid.Must(), ext)
		if err := c.SaveUploadedFile(dto.ImageData, filepath.Join(ImagesDirectory, fileName)); err != nil {
			log.Default().Printf("failed to save image %s: %s", fileName, err)
			MakeErrorResponse(c, 500, "系统内部出错，修改失败")
			return
		}
		
		user.ImageUrl = JoinUrl(ServiceUrl, "images", fileName)
	}
	//保存用户信息（不含密码）
	if !SaveUser(c, user) {
		return
	}
	//保存用户密码
	if !SavePassword(c, dto.Password, user.Username) {
		return
	}

	MakeResponse(c, 200, "修改用户信息成功")
}
//重置积分
type ResetPointsDto struct {
	Points int `json:"points" validate:"required"`
}

func ResetPointsHandler(c *gin.Context) {
	var dto ResetPointsDto
	if !CheckDto(c, &dto) {
		return
	}

	user := MustGetUser(c)
	user.Points = dto.Points
	if !SaveUser(c, user) {
		return
	}

	MakeResponse(c, 200, "更新积分成功")
}
//获取用户所有账单，账单在Redis中用List存储
func GetListHandler(c *gin.Context) {
	user := MustGetUser(c)
	//返回所有账单
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	result, err := RedisClient.LRange(ctx, GetListUserRecordsKey(user), 0, -1).Result()
	cancel()
	if err != nil && err != redis.Nil {
		log.Default().Printf("failed to call redis: %s", err)
		MakeErrorResponse(c, 500, "系统内部出错，修改失败")
		return
	}
	//将返回的result list转换为字符串，用comma分割
	data := ""
	for i, item := range result {
		data += item
		if i < len(result)-1 {
			data += ","
		}
	}
	//返回状态码200，定义媒体类型为JSON，并附带数据
	c.Data(200, gin.MIMEJSON, []byte(fmt.Sprintf(`{"code":200,"data":[%s]}`, data)))
}
//添加账单
type AddRecordDto struct {
	Type   string  `json:"type" validate:"required"`
	Tips   string  `json:"tips" validate:"required"`
	Amount float64 `json:"amount"`
	Date   string  `json:"date" validate:"required"`
}

func AddRecordHandler(c *gin.Context) {
	var dto AddRecordDto
	if !CheckDto(c, &dto) {
		return
	}

	user := MustGetUser(c)
	//解析JSON
	data, err := json.Marshal(map[string]interface{}{
		"id":     gonanoid.Must(),
		"type":   dto.Type,
		"tips":   dto.Tips,
		"amount": dto.Amount,
		"date":   dto.Date,
	})
	if err != nil {
		log.Default().Printf("failed to marshal add record dto: %s", err)
		MakeErrorResponse(c, 500, "系统内部出错，添加失败")
		return
	}
	//添加到redis
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	err = RedisClient.RPush(ctx, GetListUserRecordsKey(user), string(data)).Err()
	cancel()
	if err != nil {
		log.Default().Printf("failed to call redis: %s", err)
		MakeErrorResponse(c, 500, "系统内部出错，添加失败")
		return
	}

	MakeResponse(c, 200, "添加记录成功")
}

type DeleteRecordDto struct {
	Id string `json:"id" validate:"gte=0"`
}
//删除记录
func DeleteRecordHandler(c *gin.Context) {
	var dto DeleteRecordDto
	if !CheckDto(c, &dto) {
		return
	}

	user := MustGetUser(c)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	result, err := RedisClient.LRange(ctx, GetListUserRecordsKey(user), 0, -1).Result()
	cancel()
	if err == redis.Nil {
		MakeErrorResponse(c, 404, "此 Id 对应的记录不存在")
		return
	}
	if err != nil {
		log.Default().Printf("failed to call redis: %s", err)
		MakeErrorResponse(c, 500, "系统内部出错，删除失败")
		return
	}
	//在对象中寻找删除对象
	target := ""
	for _, data := range result {
		//遍历所有的账单
		var record map[string]interface{}
		if err := json.Unmarshal([]byte(data), &record); err != nil {
			//Json Marshal：将数据编码成json字符串
			//Json Unmarshal：将json字符串解码到相应的数据结构
			log.Default().Printf("failed to unmarshal json: %s", err)
			MakeErrorResponse(c, 500, "系统内部出错，删除失败")
			return
		}
		if record["id"] == dto.Id {
			target = data
			break
		}
	}

	if target == "" {
		MakeErrorResponse(c, 404, "此 Id 对应的记录不存在")
		return
	}

	ctx, cancel = context.WithTimeout(context.Background(), time.Second*5)
	//LRem的第三个参数count
	//count > 0 : 从表头开始向表尾搜索，移除与 VALUE 相等的元素，数量为 COUNT 。
	//count < 0 : 从表尾开始向表头搜索，移除与 VALUE 相等的元素，数量为 COUNT 的绝对值。
	//count = 0 : 移除表中所有与 VALUE 相等的值。
	err = RedisClient.LRem(ctx, GetListUserRecordsKey(user), 1, target).Err()
	cancel()
	if err != nil {
		log.Default().Printf("failed to call redis: %s", err)
		MakeErrorResponse(c, 500, "系统内部出错，删除失败")
		return
	}

	MakeResponse(c, 200, "删除成功")
}
