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

	log "github.com/sirupsen/logrus"

	cmodel "github.com/open-falcon/falcon-plus/common/model"
	"github.com/open-falcon/falcon-plus/modules/alarm/api"
	"github.com/open-falcon/falcon-plus/modules/alarm/g"
	"github.com/open-falcon/falcon-plus/modules/alarm/redi"
)

func consume(event *cmodel.Event, isHigh bool) {
	actionId := event.ActionId() //获取事件关联的事件动作ID
	if actionId <= 0 {
		return
	}

	action := api.GetAction(actionId) //根据动作ID查询API组件获取对应的具体动作定义
	if action == nil {
		return
	}

	//如果Callback字段值为1，表示有回调。调用HandleCallback处理回调逻辑，把报警的信息作为参数带上
	if action.Callback == 1 {
		HandleCallback(event, action)
	}

	//根据isHigh参数的不同，事件会被分发到不同的处理逻辑
	if isHigh {
		consumeHighEvents(event, action)
	} else {
		consumeLowEvents(event, action)
	}
}

// 高优先级的不做报警合并
func consumeHighEvents(event *cmodel.Event, action *api.Action) {
	if action.Uic == "" { //如果报警没有接收组,那么直接返回
		return
	}

	phones, mails, ims := api.ParseTeams(action.Uic) //API组件查询解析告警组成员的通知联系信息

	//生成报警内容
	smsContent := GenerateSmsContent(event)
	mailContent := GenerateMailContent(event)
	imContent := GenerateIMContent(event)

	//redi.WriteSms等方法就是将报警内容lpush到不同通道的发送队列中
	// <=P2 才发送短信
	if event.Priority() < 3 {
		redi.WriteSms(phones, smsContent)
	}

	redi.WriteIM(ims, imContent)
	redi.WriteMail(mails, smsContent, mailContent)

}

// 低优先级的做报警合并
func consumeLowEvents(event *cmodel.Event, action *api.Action) {
	if action.Uic == "" {
		return
	}

	//parseuser函数将event转换为合并消息 写入中间队列
	// <=P2 才发送短信
	if event.Priority() < 3 {
		ParseUserSms(event, action)
	}

	ParseUserIm(event, action)
	ParseUserMail(event, action)
}

func ParseUserSms(event *cmodel.Event, action *api.Action) {
	userMap := api.GetUsers(action.Uic)

	content := GenerateSmsContent(event)
	metric := event.Metric()
	status := event.Status
	priority := event.Priority()

	queue := g.Config().Redis.UserSmsQueue

	rc := g.RedisConnPool.Get()
	defer rc.Close()

	for _, user := range userMap {
		dto := SmsDto{
			Priority: priority,
			Metric:   metric,
			Content:  content,
			Phone:    user.Phone,
			Status:   status,
		}
		bs, err := json.Marshal(dto)
		if err != nil {
			log.Error("json marshal SmsDto fail:", err)
			continue
		}

		_, err = rc.Do("LPUSH", queue, string(bs))
		if err != nil {
			log.Error("LPUSH redis", queue, "fail:", err, "dto:", string(bs))
		}
	}
}

func ParseUserMail(event *cmodel.Event, action *api.Action) {
	userMap := api.GetUsers(action.Uic) //获取用户邮箱映射

	metric := event.Metric() //从event中提取监控指标metric
	//使用GenerateSmsContent(event)和GenerateMailContent(event)函数生成邮件的主题和内容
	subject := GenerateSmsContent(event)
	content := GenerateMailContent(event)
	status := event.Status
	priority := event.Priority()

	queue := g.Config().Redis.UserMailQueue

	rc := g.RedisConnPool.Get()
	defer rc.Close()

	//构建邮件DTO
	for _, user := range userMap {
		dto := MailDto{
			Priority: priority,
			Metric:   metric,
			Subject:  subject,
			Content:  content,
			Email:    user.Email,
			Status:   status,
		}
		//对每个用户的MailDto实例进行JSON序列化
		bs, err := json.Marshal(dto)
		if err != nil {
			log.Error("json marshal MailDto fail:", err)
			continue
		}

		//推送至Redis队列，此时低优先级的报警存在于配置文件中的中间队列名称的redis队列中 /queue/user/mail
		_, err = rc.Do("LPUSH", queue, string(bs))
		if err != nil {
			log.Error("LPUSH redis", queue, "fail:", err, "dto:", string(bs))
		}
	}
}

func ParseUserIm(event *cmodel.Event, action *api.Action) {
	userMap := api.GetUsers(action.Uic)

	content := GenerateIMContent(event)
	metric := event.Metric()
	status := event.Status
	priority := event.Priority()

	queue := g.Config().Redis.UserIMQueue

	rc := g.RedisConnPool.Get()
	defer rc.Close()

	for _, user := range userMap {
		dto := ImDto{
			Priority: priority,
			Metric:   metric,
			Content:  content,
			IM:       user.IM,
			Status:   status,
		}
		bs, err := json.Marshal(dto)
		if err != nil {
			log.Error("json marshal ImDto fail:", err)
			continue
		}

		_, err = rc.Do("LPUSH", queue, string(bs))
		if err != nil {
			log.Error("LPUSH redis", queue, "fail:", err, "dto:", string(bs))
		}
	}
}
