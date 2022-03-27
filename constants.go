package main

import "fmt"

const HASH_USER_PASSWORDS = "mosad_server_user_passwords"
const HASH_USER_DATA = "mosad_server_user_data"
const LIST_USER_RECORDS = "mosad_server_user_record"
//用于返回某用户的账单的list名，由于每个用户都有一个list，所以list名各不相同，而维护用户信息/密码的哈希表只有一个
func GetListUserRecordsKey(user *User) string {
	return fmt.Sprintf("%s_%s", LIST_USER_RECORDS, user.Username)
}
