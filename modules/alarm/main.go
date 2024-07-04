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

package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
	"github.com/open-falcon/falcon-plus/modules/alarm/cron"
	"github.com/open-falcon/falcon-plus/modules/alarm/g"
	"github.com/open-falcon/falcon-plus/modules/alarm/http"
	"github.com/open-falcon/falcon-plus/modules/alarm/model"
)

func main() {
	g.BinaryName = BinaryName
	g.Version = Version
	g.GitCommit = GitCommit

	cfg := flag.String("c", "cfg.json", "configuration file")
	version := flag.Bool("v", false, "show version")
	help := flag.Bool("h", false, "help")
	flag.Parse()

	if *version {
		fmt.Printf("Open-Falcon %s version %s, build %s\n", BinaryName, Version, GitCommit)
		os.Exit(0)
	}

	if *help {
		flag.Usage()
		os.Exit(0)
	}

	g.ParseConfig(*cfg) //全局配置文件解析

	g.InitLog(g.Config().LogLevel)
	if g.Config().LogLevel != "debug" {
		gin.SetMode(gin.ReleaseMode)
	}

	g.InitRedisConnPool()   //初始化Redis连接池
	model.InitDatabase()    //初始化数据库ORM
	cron.InitSenderWorker() //初始化发送Channel

	go http.Start()         //启动HTTP服务,API服务监听与处理
	go cron.ReadHighEvent() //处理高优先级事件队列
	go cron.ReadLowEvent()  //处理低优先级事件队列

	go cron.CombineSms()  //合并SMS内容
	go cron.CombineMail() //合并MAIL内容
	go cron.CombineIM()   //合并IM内容

	go cron.ConsumeIM()   //发送事件IM
	go cron.ConsumeSms()  //发送事件Sms
	go cron.ConsumeMail() //发送事件Mail

	go cron.CleanExpiredEvent() //清理过期事件信息

	// 注册系统信号syscall.SIGTERM，退出释放资源
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		fmt.Println()
		g.RedisConnPool.Close()
		os.Exit(0)
	}()

	select {}
}
