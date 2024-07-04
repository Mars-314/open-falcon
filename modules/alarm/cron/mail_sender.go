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
	"time"

	"github.com/open-falcon/falcon-plus/modules/alarm/g"
	"github.com/open-falcon/falcon-plus/modules/alarm/model"
	"github.com/open-falcon/falcon-plus/modules/alarm/redi"
	log "github.com/sirupsen/logrus"
	"github.com/toolkits/net/httplib"
)

func ConsumeMail() {
	//调用redi.PopAllMail()从Redis队列中弹出所有Mail消息实体。
	//如果队列为空，函数不会立即返回，而是\让当前goroutine暂停200毫秒后继续检查，实现了一种简单的退避策略
	for {
		L := redi.PopAllMail()
		if len(L) == 0 {
			time.Sleep(time.Millisecond * 200)
			continue
		}
		SendMailList(L)
	}
}

func SendMailList(L []*model.Mail) {
	for _, mail := range L {
		MailWorkerChan <- 1 //对于每个Mail消息，先向MailWorkerChan通道发送一个信号（数值1），用于限制并发量
		go SendMail(mail)   //启动一个新的goroutine执行SendIM函数来发送单个Mail消息
	}
}

func SendMail(mail *model.Mail) {
	defer func() {
		<-MailWorkerChan
	}()

	//根据配置获取IM服务API的URL
	url := g.Config().Api.Mail
	//使用httplib.Post创建一个HTTP POST请求，设置超时时间为5秒连接超时，30秒读取超时
	r := httplib.Post(url).SetTimeout(5*time.Second, 30*time.Second)
	r.Param("tos", mail.Tos)
	r.Param("subject", mail.Subject)
	r.Param("content", mail.Content)
	resp, err := r.String()
	if err != nil {
		log.Errorf("send mail fail, receiver:%s, subject:%s, content:%s, error:%v", mail.Tos, mail.Subject, mail.Content, err)
	}

	//使用defer语句确保在函数退出前从MailWorkerChan通道接收一个信号，这与在SendIMList中发送的信号相匹配，从而控制并发工作goroutine的数量，避免无限制的增长
	log.Debugf("send mail:%v, resp:%v, url:%s", mail, resp, url)
}
