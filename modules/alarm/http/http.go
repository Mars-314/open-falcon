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

// 启动一个包含基础健康检查、版本信息查询以及工作目录查询等功能的HTTP服务
package http

import (
	"log"

	"github.com/gin-gonic/gin"
	"github.com/open-falcon/falcon-plus/modules/alarm/g"
)

func Start() {
	//检查全局配置中的Http.Enabled字段。如果HTTP服务未启用，则直接返回。
	if !g.Config().Http.Enabled {
		return
	}

	// 如果HTTP服务启用，尝试从配置中获取监听地址。如果地址为空字符串，则同样直接返回，不启动服务
	addr := g.Config().Http.Listen
	if addr == "" {
		return
	}

	//注册路由处理函数
	r := gin.Default()         //初始化路由: 使用gin.Default()创建一个默认的Gin路由器实例
	r.GET("/version", Version) // 注册一个GET类型的路由/version，当访问这个URL时，会调用Version函数来处理请求，用于返回服务的版本信息
	r.GET("/health", Health)   //注册一个健康检查路由/health，调用Health函数，用于检查服务的健康状态
	r.GET("/workdir", Workdir) //注册一个获取工作目录信息的路由/workdir，由Workdir函数处理，用于返回服务的工作目录路径
	r.Run(addr)                //=启动HTTP服务器，监听之前获取的地址addr。服务器现在开始接收并处理指定地址上的HTTP请求

	log.Println("http listening", addr)
}
