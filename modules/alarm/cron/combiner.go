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

package cron

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/garyburd/redigo/redis"
	"github.com/open-falcon/falcon-plus/modules/alarm/api"
	"github.com/open-falcon/falcon-plus/modules/alarm/g"
	"github.com/open-falcon/falcon-plus/modules/alarm/redi"
	log "github.com/sirupsen/logrus"
)

func CombineSms() {
	for {
		// 每分钟读取处理一次
		time.Sleep(time.Minute)
		combineSms()
	}
}

func CombineMail() {
	for {
		// 每分钟读取处理一次
		time.Sleep(time.Minute)
		combineMail()
	}
}

func CombineIM() {
	for {
		// 每分钟读取处理一次
		time.Sleep(time.Minute)
		combineIM()
	}
}

func combineMail() {
	dtos := popAllMailDto() //Mail队列"/queue/user/mail" RPOP所有事件邮件内容信息
	count := len(dtos)
	if count == 0 {
		return
	}

	//初始化一个名为dtoMap的映射，键为一个组合字符串，由邮件的优先级、状态、目标邮箱和监控指标拼接而成，值为一个MailDto切片。
	dtoMap := make(map[string][]*MailDto)
	for i := 0; i < count; i++ {
		key := fmt.Sprintf("%d%s%s%s", dtos[i].Priority, dtos[i].Status, dtos[i].Email, dtos[i].Metric)
		//遍历所有邮件DTO，根据前面定义的键规则生成映射键，并将对应的邮件DTO加入到相应的切片中。如果映射中已有该键，则直接追加；否则，创建新的切片并添加。
		if _, ok := dtoMap[key]; ok {
			dtoMap[key] = append(dtoMap[key], dtos[i])
		} else {
			dtoMap[key] = []*MailDto{dtos[i]}
		}
	}

	// 不要在这处理，继续写回redis，否则重启alarm很容易丢数据
	//对于映射中的每一个切片，如果长度为1，说明没有需要合并的邮件，直接调用redi.WriteMail发送单个邮件。
	//如果长度大于1，则需要合并邮件内容。生成一个新的邮件主题，格式为[P优先级][状态] 数量 指标，其中数量表示合并了多少封邮件。然后，遍历切片，将每封邮件的内容收集到一个新的字符串切片中，并用换行符连接这些内容形成最终邮件正文。
	//记录一条调试日志，包含合并后的邮件主题和内容，便于跟踪和调试。
	//最后，使用redi.WriteMail发送合并后的邮件到第一个邮件DTO的邮箱地址。
	for _, arr := range dtoMap {
		size := len(arr)
		if size == 1 {
			redi.WriteMail([]string{arr[0].Email}, arr[0].Subject, arr[0].Content)
			continue
		}

		subject := fmt.Sprintf("[P%d][%s] %d %s", arr[0].Priority, arr[0].Status, size, arr[0].Metric)
		contentArr := make([]string, size)
		for i := 0; i < size; i++ {
			contentArr[i] = arr[i].Content
		}
		content := strings.Join(contentArr, "\r\n")

		log.Debugf("combined mail subject:%s, content:%s", subject, content)
		redi.WriteMail([]string{arr[0].Email}, subject, content)
	}
}

func combineIM() {
	dtos := popAllImDto() //IM队列"/queue/user/im" RPOP所有事件im内容信息DTO
	count := len(dtos)
	if count == 0 {
		return
	}

	//初始化一个名为dtoMap的映射，其中键由IM的优先级、状态、IM账号和监控指标构成
	dtoMap := make(map[string][]*ImDto)
	for i := 0; i < count; i++ {
		//遍历所有邮件DTO，根据前面定义的键规则生成映射键，并将对应的邮件DTO加入到相应的切片中。如果映射中已有该键，则直接追加；否则，创建新的切片并添加。
		key := fmt.Sprintf("%d%s%s%s", dtos[i].Priority, dtos[i].Status, dtos[i].IM, dtos[i].Metric)
		if _, ok := dtoMap[key]; ok {
			dtoMap[key] = append(dtoMap[key], dtos[i])
		} else {
			dtoMap[key] = []*ImDto{dtos[i]}
		}
	}

	//对于每个分组，首先判断其长度。如果长度为1，说明不需要合并，直接调用redi.WriteIM发送单条IM。
	//对于多条IM内容，将内容合并成一个字符串数组，然后用特殊分隔符,,连接这些内容。接下来，尝试从第一条IM内容中提取示例(eg)信息。
	//之后，尝试为合并的IM内容生成一个短链接，使用api.LinkToSMS(content)方法。如果生成短链接失败或返回空链接，构建一条不含链接的IM消息，包含错误信息、优先级、状态、数量和示例信息，提示用户查看邮件获取详细信息。
	//如果短链接生成成功，构建包含短链接的IM消息，引导用户通过链接查看具体的报警详情。消息格式包括优先级、状态、数量、监控指标、示例信息和短链接地址。
	//记录日志，如果短链接生成失败则记录错误信息，成功则记录合并后的IM消息内容。
	for _, arr := range dtoMap {
		size := len(arr)
		if size == 1 {
			redi.WriteIM([]string{arr[0].IM}, arr[0].Content)
			continue
		}

		// 把多个im内容写入数据库，只给用户提供一个链接
		contentArr := make([]string, size)
		for i := 0; i < size; i++ {
			contentArr[i] = arr[i].Content
		}
		content := strings.Join(contentArr, ",,")

		first := arr[0].Content
		t := strings.Split(first, "][")
		eg := ""
		if len(t) >= 3 {
			eg = t[2]
		}

		path, err := api.LinkToSMS(content)
		chat := ""
		if err != nil || path == "" {
			chat = fmt.Sprintf("[P%d][%s] %d %s.  e.g. %s. detail in email", arr[0].Priority, arr[0].Status, size, arr[0].Metric, eg)
			log.Error("create short link fail", err)
		} else {
			chat = fmt.Sprintf("[P%d][%s] %d %s e.g. %s %s/portal/links/%s ",
				arr[0].Priority, arr[0].Status, size, arr[0].Metric, eg, g.Config().Api.Dashboard, path)
			log.Debugf("combined im is:%s", chat)
		}

		//使用redi.WriteIM将合并后的IM消息发送到第一个IM账号
		redi.WriteIM([]string{arr[0].IM}, chat)
	}
}

func combineSms() {
	dtos := popAllSmsDto()
	count := len(dtos)
	if count == 0 {
		return
	}

	dtoMap := make(map[string][]*SmsDto)
	for i := 0; i < count; i++ {
		key := fmt.Sprintf("%d%s%s%s", dtos[i].Priority, dtos[i].Status, dtos[i].Phone, dtos[i].Metric)
		if _, ok := dtoMap[key]; ok {
			dtoMap[key] = append(dtoMap[key], dtos[i])
		} else {
			dtoMap[key] = []*SmsDto{dtos[i]}
		}
	}

	for _, arr := range dtoMap {
		size := len(arr)
		if size == 1 {
			redi.WriteSms([]string{arr[0].Phone}, arr[0].Content)
			continue
		}

		// 把多个sms内容写入数据库，只给用户提供一个链接
		contentArr := make([]string, size)
		for i := 0; i < size; i++ {
			contentArr[i] = arr[i].Content
		}
		content := strings.Join(contentArr, ",,")

		first := arr[0].Content
		t := strings.Split(first, "][")
		eg := ""
		if len(t) >= 3 {
			eg = t[2]
		}

		path, err := api.LinkToSMS(content)
		sms := ""
		if err != nil || path == "" {
			sms = fmt.Sprintf("[P%d][%s] %d %s.  e.g. %s. detail in email", arr[0].Priority, arr[0].Status, size, arr[0].Metric, eg)
			log.Error("get short link fail", err)
		} else {
			sms = fmt.Sprintf("[P%d][%s] %d %s e.g. %s %s/portal/links/%s ",
				arr[0].Priority, arr[0].Status, size, arr[0].Metric, eg, g.Config().Api.Dashboard, path)
			log.Debugf("combined sms is:%s", sms)
		}

		redi.WriteSms([]string{arr[0].Phone}, sms)
	}
}

func popAllSmsDto() []*SmsDto {
	ret := []*SmsDto{}
	queue := g.Config().Redis.UserSmsQueue

	rc := g.RedisConnPool.Get()
	defer rc.Close()

	for {
		reply, err := redis.String(rc.Do("RPOP", queue))
		if err != nil {
			if err != redis.ErrNil {
				log.Error("get SmsDto fail", err)
			}
			break
		}

		if reply == "" || reply == "nil" {
			continue
		}

		var smsDto SmsDto
		err = json.Unmarshal([]byte(reply), &smsDto)
		if err != nil {
			log.Errorf("json unmarshal SmsDto: %s fail: %v", reply, err)
			continue
		}

		ret = append(ret, &smsDto)
	}

	return ret
}

// 低优先级的报警存在于配置文件中的中间队列名称的redis队列中/queue/user/mail
// 所有事件邮件内容信息出队
func popAllMailDto() []*MailDto {
	//定义一个指向MailDto结构体的切片ret，用于存储从Redis队列中弹出的所有邮件DTO
	ret := []*MailDto{}
	queue := g.Config().Redis.UserMailQueue //队列"/queue/user/mail"

	//获取Redis用户邮件队列的名字，并从全局Redis连接池中获取一个连接。通过defer rc.Close()确保连接在函数结束时被关闭
	rc := g.RedisConnPool.Get()
	defer rc.Close()

	//使用redis.String(rc.Do("RPOP", queue))执行Redis的RPOP命令。此命令会移除并返回队列的最后一个元素。
	//如果执行时发生错误，且错误不是redis.ErrNil（表示队列为空），则记录错误并跳出循环。如果是redis.ErrNil，则说明队列已空，这是正常情况，无需处理。
	//如果回复是空字符串或"nil"，说明弹出了一个空元素，这种情况通常是因为队列刚好变空，因此直接跳过本次循环继续尝试。
	for {
		reply, err := redis.String(rc.Do("RPOP", queue))
		if err != nil {
			if err != redis.ErrNil {
				log.Error("get MailDto fail", err)
			}
			break
		}

		if reply == "" || reply == "nil" {
			continue
		}

		//将回复的JSON字符串反序列化为MailDto结构体
		var mailDto MailDto
		err = json.Unmarshal([]byte(reply), &mailDto)
		if err != nil {
			log.Errorf("json unmarshal MailDto: %s fail: %v", reply, err)
			continue
		}

		// 成功反序列化的MailDto实例被追加到ret切片中
		ret = append(ret, &mailDto)
	}

	return ret
}

func popAllImDto() []*ImDto {
	ret := []*ImDto{}
	queue := g.Config().Redis.UserIMQueue

	rc := g.RedisConnPool.Get()
	defer rc.Close()

	for {
		reply, err := redis.String(rc.Do("RPOP", queue))
		if err != nil {
			if err != redis.ErrNil {
				log.Error("get ImDto fail", err)
			}
			break
		}

		if reply == "" || reply == "nil" {
			continue
		}

		var imDto ImDto
		err = json.Unmarshal([]byte(reply), &imDto)
		if err != nil {
			log.Errorf("json unmarshal imDto: %s fail: %v", reply, err)
			continue
		}

		ret = append(ret, &imDto)
	}

	return ret
}
