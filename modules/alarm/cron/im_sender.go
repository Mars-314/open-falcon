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
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/open-falcon/falcon-plus/modules/alarm/g"
	"github.com/open-falcon/falcon-plus/modules/alarm/model"
	"github.com/open-falcon/falcon-plus/modules/alarm/redi"
	log "github.com/sirupsen/logrus"
	"github.com/toolkits/net/httplib"
)

type DingTalkMessage struct {
	MsgType string `json:"msgtype"`
	Text    struct {
		Content string `json:"content"`
	} `json:"text"`
}

func ConsumeIM() {
	for {
		L := redi.PopAllIM()
		if len(L) == 0 {
			time.Sleep(time.Millisecond * 200)
			continue
		}
		SendIMList(L)
	}
}

func SendIMList(L []*model.IM) {
	for _, im := range L {
		IMWorkerChan <- 1
		go SendDING(im)
	}
}

func SendIM(im *model.IM) {
	defer func() {
		<-IMWorkerChan
	}()

	url := g.Config().Api.IM
	r := httplib.Post(url).SetTimeout(5*time.Second, 30*time.Second)
	r.Param("tos", im.Tos)
	r.Param("content", im.Content)
	resp, err := r.String()
	if err != nil {
		log.Println("send im fail, tos:", im.Tos, ", content:", im.Content, ", error:", err)
		log.Errorf("send im fail, tos:%s, content:%s, error:%v", im.Tos, im.Content, err)
	}

	log.Println("send im:", im, ", resp:", resp, ", url:", url)
	log.Debugf("send im:%v, resp:%v, url:%s", im, resp, url)
}

func SendDING(im *model.IM) {
	defer func() {
		<-IMWorkerChan
	}()

	dingTalkMessage := DingTalkMessage{
		MsgType: "text",
		Text: struct {
			Content string `json:"content"`
		}{
			Content: im.Content,
		},
	}
	// 序列化消息体为JSON
	jsonValue, err := json.Marshal(dingTalkMessage)
	if err != nil {
		log.Errorf("序列化消息体为JSON fail, dingTalkMessage:%s, error:%v", dingTalkMessage, err)
	}

	// 发送HTTP POST请求
	resp, err := http.Post(im.Tos, "application/json", bytes.NewBuffer(jsonValue))
	if err != nil {
		log.Errorf("send dingding fail, tos:%s, content:%s, dingTalkMessage:%s, error:%v,", im.Tos, im.Content, dingTalkMessage, err)
	}
	defer resp.Body.Close()

	// 读取响应体
	body, _ := ioutil.ReadAll(resp.Body)
	fmt.Printf("Response: %s\n", body)
	// 检查响应状态码
	if resp.StatusCode != http.StatusOK {
		fmt.Printf("HTTP Status: %s, Body: %s", resp.Status, body)
	}

	log.Debugf("send im:%v, resp:%v, url:%s", im, resp, im.Tos)
}
