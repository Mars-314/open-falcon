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
	"fmt"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/open-falcon/falcon-plus/common/model"
	"github.com/open-falcon/falcon-plus/common/utils"
	"github.com/open-falcon/falcon-plus/modules/alarm/api"
	"github.com/open-falcon/falcon-plus/modules/alarm/redi"
	"github.com/toolkits/net/httplib"
)

func HandleCallback(event *model.Event, action *api.Action) {

	teams := action.Uic //从给定的action中提取Uic
	phones := []string{}
	mails := []string{}
	ims := []string{}

	//调用api.ParseTeams根据团队标识解析出成员的sms,mail,im
	if teams != "" {
		phones, mails, ims = api.ParseTeams(teams)
		smsContent := GenerateSmsContent(event)
		mailContent := GenerateMailContent(event)
		imContent := GenerateIMContent(event)
		if action.BeforeCallbackSms == 1 { //如果action.BeforeCallbackSms为1，表示需要在回调执行前发送sms和im
			redi.WriteSms(phones, smsContent)
			redi.WriteIM(ims, imContent)
		}

		if action.BeforeCallbackMail == 1 { //同上
			redi.WriteMail(mails, smsContent, mailContent)
		}
	}

	message := Callback(event, action) //调用Callback函数执行回调逻辑，CallBack URL配置执行

	if teams != "" {
		if action.AfterCallbackSms == 1 { //如果action.AfterCallbackSms为1，表示需要在回调执行后发送sms和im
			redi.WriteSms(phones, message)
			redi.WriteIM(ims, message)
		}

		if action.AfterCallbackMail == 1 { //同上
			redi.WriteMail(mails, message, message)
		}
	}

}

func Callback(event *model.Event, action *api.Action) string {
	if action.Url == "" {
		return "callback url is blank"
	}

	//遍历标签并将其格式化为key:value的形式
	L := make([]string, 0)
	if len(event.PushedTags) > 0 {
		for k, v := range event.PushedTags {
			L = append(L, fmt.Sprintf("%s:%s", k, v))
		}
	}

	//将标签用逗号连接成一个字符串tags
	tags := ""
	if len(L) > 0 {
		tags = strings.Join(L, ",")
	}

	//使用httplib.Get(action.Url)初始化GET请求，并设置3秒连接超时，20秒读取超时
	req := httplib.Get(action.Url).SetTimeout(3*time.Second, 20*time.Second)

	//根据event对象的属性，向请求中添加多个参数。这里使用了fmt.Sprintf来格式化数值类型的参数，并使用utils.ReadableFloat来格式化浮点数为易读格式
	req.Param("endpoint", event.Endpoint)
	req.Param("metric", event.Metric())
	req.Param("status", event.Status)
	req.Param("step", fmt.Sprintf("%d", event.CurrentStep))
	req.Param("priority", fmt.Sprintf("%d", event.Priority()))
	req.Param("time", event.FormattedTime())
	req.Param("tpl_id", fmt.Sprintf("%d", event.TplId()))
	req.Param("exp_id", fmt.Sprintf("%d", event.ExpressionId()))
	req.Param("stra_id", fmt.Sprintf("%d", event.StrategyId()))
	req.Param("left_value", utils.ReadableFloat(event.LeftValue))
	req.Param("tags", tags)

	//调用req.String()发送请求并获取响应字符串
	resp, e := req.String()

	success := "success"
	if e != nil {
		log.Errorf("callback fail, action:%v, event:%s, error:%s", action, event.String(), e.Error())
		success = fmt.Sprintf("fail:%s", e.Error())
	}
	//根据请求的成功或失败，构造一条包含原始请求URL、操作结果（成功或失败原因）、以及实际响应内容的消息。
	message := fmt.Sprintf("curl %s %s. resp: %s", action.Url, success, resp)
	//不论请求成功还是失败，都通过log.Debugf记录一条调试信息，包含回调的URL、事件详情和响应内容
	log.Debugf("callback to url:%s, event:%s, resp:%s", action.Url, event.String(), resp)

	return message
}
