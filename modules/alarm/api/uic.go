// Copyright 2017 Xiaomi, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package api

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/open-falcon/falcon-plus/modules/alarm/g"
	"github.com/open-falcon/falcon-plus/modules/api/app/model/uic"
	log "github.com/sirupsen/logrus"
	"github.com/toolkits/container/set"
	"github.com/toolkits/net/httplib"
)

type APIGetTeamOutput struct {
	uic.Team
	Users       []*uic.User `json:"users"`
	TeamCreator string      `json:"creator_name"`
}

type UsersCache struct {
	sync.RWMutex
	M map[string][]*uic.User
}

var Users = &UsersCache{M: make(map[string][]*uic.User)}

func (this *UsersCache) Get(team string) []*uic.User {
	this.RLock()
	defer this.RUnlock()
	val, exists := this.M[team]
	if !exists {
		return nil
	}

	return val
}

func (this *UsersCache) Set(team string, users []*uic.User) {
	this.Lock()
	defer this.Unlock()
	this.M[team] = users
}

// 通过成员组查询成员
func UsersOf(team string) []*uic.User {
	users := CurlUic(team)

	if users != nil {
		Users.Set(team, users)
	} else {
		users = Users.Get(team)
	}

	return users
}

// 通过成员组信息查询用户，获取用户信息map
func GetUsers(teams string) map[string]*uic.User {
	userMap := make(map[string]*uic.User)
	arr := strings.Split(teams, ",")
	for _, team := range arr {
		if team == "" {
			continue
		}

		users := UsersOf(team)
		if users == nil {
			continue
		}

		for _, user := range users {
			userMap[user.Name] = user
		}
	}
	return userMap
}

// 通过API查询解析维护人员组成员phones, emails, IM
func ParseTeams(teams string) ([]string, []string, []string) {
	if teams == "" {
		return []string{}, []string{}, []string{}
	}

	userMap := GetUsers(teams)
	phoneSet := set.NewStringSet()
	mailSet := set.NewStringSet()
	imSet := set.NewStringSet()
	for _, user := range userMap {
		if user.Phone != "" {
			phoneSet.Add(user.Phone)
		}
		if user.Email != "" {
			mailSet.Add(user.Email)
		}
		if user.IM != "" {
			imSet.Add(user.IM)
		}
	}
	return phoneSet.ToSlice(), mailSet.ToSlice(), imSet.ToSlice()
}

// API组件接口，CURL HTTP访问与响应数据处理
// 从UIC系统中根据团队名称获取该团队的所有用户信息
func CurlUic(team string) []*uic.User {
	if team == "" {
		return []*uic.User{}
	}

	//根据全局配置拼接出请求UIC系统的URI，格式为{PlusApi}/api/v1/team/name/{team}，其中{team}为传入的团队名称
	uri := fmt.Sprintf("%s/api/v1/team/name/%s", g.Config().Api.PlusApi, team)
	//使用httplib.Get(uri)发起一个GET请求，并设置HTTP请求的超时时间为2秒连接超时，10秒读取超时
	req := httplib.Get(uri).SetTimeout(2*time.Second, 10*time.Second)
	//将鉴权信息（包含API名称和签名）以JSON格式序列化，并设置到请求头Apitoken中。鉴权信息由配置中的g.Config().Api.PlusApiToken提供
	token, _ := json.Marshal(map[string]string{
		"name": "falcon-alarm",
		"sig":  g.Config().Api.PlusApiToken,
	})
	req.Header("Apitoken", string(token))

	//使用req.ToJson发送请求并将响应体反序列化到team_users变量中，该变量类型为APIGetTeamOutput，用于接收UIC系统返回的关于团队用户信息的数据
	var team_users APIGetTeamOutput
	err := req.ToJson(&team_users)
	if err != nil {
		log.Errorf("curl %s fail: %v", uri, err)
		return nil
	}

	return team_users.Users
}
