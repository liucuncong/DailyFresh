package main

import (
	_ "DailyFresh/models"
	_ "DailyFresh/routers"

	"github.com/astaxie/beego"
)

func main() {
	beego.Run()
}
