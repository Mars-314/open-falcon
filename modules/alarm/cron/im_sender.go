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
		go SendIM(im)
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
