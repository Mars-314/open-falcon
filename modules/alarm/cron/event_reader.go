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
	"time"

	"github.com/garyburd/redigo/redis"
	cmodel "github.com/open-falcon/falcon-plus/common/model"
	"github.com/open-falcon/falcon-plus/modules/alarm/g"
	eventmodel "github.com/open-falcon/falcon-plus/modules/alarm/model/event"
	log "github.com/sirupsen/logrus"
)

//highQueues中配置的几个event队列中的事件是不会做报警合并的，因为那些是高优先级的报警，报警合并只是针对lowQueues中的事件。如果所有的事件都不想做报警合并，就把所有的event队列都配置到highQueues中即可

//每个级别的报警都会对应不同的redis队列，alarm去读取这个数据的时候先读取P0的数据，再读取P1的数据，最后读取P5的数据。于是：用了redis的brpop指令

//从Redis读取高优先级队列事件信息
//解析事件信息（action/callback/teams/user:phone、im、mail等）
//根据事件信息生成IM、SMS、MAIL内容存入Redis队列

func ReadHighEvent() {
	queues := g.Config().Redis.HighQueues //默认event:p0､p1､p2
	if len(queues) == 0 {
		return
	}

	//函数进入一个无限循环，不断从Redis队列中读取高优先级事件并处理。这种设计是为了持续不断地监控和处理新的事件
	//传入的是包含多个高优先级的队列的列表比如[p0,p1,p2]，那么总是先pop完event:p0的队列，然后才是p1，p2
	for {
		event, err := popEvent(queues) //事件出列
		if err != nil {
			time.Sleep(time.Second)
			continue
		}
		consume(event, true) //处理事件
	}
}

func ReadLowEvent() {
	queues := g.Config().Redis.LowQueues
	if len(queues) == 0 {
		return
	}

	for {
		event, err := popEvent(queues)
		if err != nil {
			time.Sleep(time.Second)
			continue
		}
		consume(event, false)
	}
}

// 从Redis队列中弹出报警事件，并将其解析、存储到数据库中
func popEvent(queues []string) (*cmodel.Event, error) {

	count := len(queues)

	//函数接收一个字符串切片参数queues，这个切片包含了要从中弹出事件的一个或多个Redis队列的名称
	//初始化一个参数数组params，长度为queues的长度加1。遍历queues并将每个队列名称添加到params中，最后添加一个0作为BRPOP命令的超时参数，表示阻塞等待直到有元素可弹出。
	params := make([]interface{}, count+1)
	for i := 0; i < count; i++ {
		params[i] = queues[i]
	}
	params[count] = 0

	//从连接池g.RedisConnPool中获取一个Redis连接。defer rc.Close()确保函数执行完毕后关闭这个连接。
	//g.RedisConnPool在falcon-plus\modules\alarm\g\redis.go中读取配置文件生成
	rc := g.RedisConnPool.Get()
	defer rc.Close()

	//执行Redis的BRPOP命令，该命令从队列的右侧弹出最后一个元素。
	//因为传入了多个队列，Redis会按照顺序检查这些队列，从第一个非空队列中弹出元素。
	//命令成功后，reply是一个包含两个元素的字符串切片，其中reply[0]是队列名称，reply[1]是弹出的元素,即事件JSON字符串
	reply, err := redis.Strings(rc.Do("BRPOP", params...))
	if err != nil {
		log.Errorf("get alarm event from redis fail: %v", err)
		return nil, err
	}

	//将JSON格式的事件字符串反序列化为cmodel.Event结构体
	var event cmodel.Event
	err = json.Unmarshal([]byte(reply[1]), &event)
	if err != nil {
		log.Errorf("parse alarm event fail: %v", err)
		return nil, err
	}

	log.Debugf("pop event: %s", event.String())

	//insert event into database
	//将事件实体存储到数据库中
	eventmodel.InsertEvent(&event)
	// events no longer saved in memory

	return &event, nil
}
