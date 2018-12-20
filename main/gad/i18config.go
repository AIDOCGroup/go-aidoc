package main

import "gopkg.in/urfave/cli.v1"

//多语言配置
var (
	i18Flag			= cli.StringFlag{
		Name:"lang",
		Usage:"多语言" ,
	}
)