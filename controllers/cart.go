package controllers

import (
	"github.com/astaxie/beego"
	"github.com/gomodule/redigo/redis"
	"DailyFresh/models"
	"github.com/astaxie/beego/orm"
	"strconv"
	)

type CartController struct {
	beego.Controller
}

//处理添加购物车
func (this *CartController )HandleAddCart()  {
	//1.获取数据
	skuid,err1 := this.GetInt("skuid")
	count,err2 := this.GetInt("count")

	resp := make(map[string]interface{})
	defer this.ServeJSON()

	//2.校验数据
	if err1 != nil || err2 != nil{
		resp["code"] = 1
		resp["msg"] = "传递的数据不正确"
		this.Data["json"] = resp
		return

	}

	username := this.GetSession("username")
	if username == nil{
		resp["code"] = 2
		resp["msg"] = "用户未登录"
		this.Data["json"] = resp
		return

	}
	//3.处理数据
	//获取用户id
	o := orm.NewOrm()
	var user models.User
	user.Name = username.(string)
	o.Read(&user,"Name")


	//购物车数据存在redis中，用hash
	conn,err := redis.Dial("tcp","192.168.100.9:6379")
	if err != nil{
		beego.Info("redis数据库连接错误")
		return
	}
	defer conn.Close()

	//先获取原来的数量，然后把数量加起来
	preCount,_ := redis.Int(conn.Do("hget","cart_"+strconv.Itoa(user.Id),skuid))

	conn.Do("hset","cart_"+strconv.Itoa(user.Id),skuid,count+preCount)

	res,err := conn.Do("hlen","cart_"+strconv.Itoa(user.Id))
	// 回复助手函数
	cartCount,_ := redis.Int(res,err)


	//01.map做为json的容器
	//resp := make(map[string]interface{})
	resp["code"] = 5
	resp["msg"] = "OK"
	resp["cartCount"] = cartCount

	//02.指定json格式
	this.Data["json"] = resp
	//03.返回json
	//this.ServeJSON()

	//4.返回json格式的数据


}

//获取购物车数量的函数
func GetCartCount(this *beego.Controller) int {
	//从redis中获取购物车数量
	username := this.GetSession("username")
	if username == nil{
		return 0
	}
	o := orm.NewOrm()
	var user models.User
	user.Name = username.(string)
	o.Read(&user,"Name")

	conn,err := redis.Dial("tcp","192.168.100.9:6379")
	if err != nil {
		return 0
	}
	defer conn.Close()

	res,err := conn.Do("hlen","cart_"+strconv.Itoa(user.Id))
	cartCount,_ := redis.Int(res,err)
	return cartCount

}

//展示购物车页面
func (this *CartController)ShowMyCart()  {
	//用户信息
	username := GetUserName(&this.Controller)
	o := orm.NewOrm()
	var user models.User
	user.Name = username
	o.Read(&user,"Name")

	//从redis获取购物车信息
	conn,err := redis.Dial("tcp","192.168.100.9:6379")
	if err != nil {
		beego.Info("redis连接失败")
		return
	}
	defer conn.Close()

	goodsMap,_ := redis.IntMap(conn.Do("hgetall","cart_"+strconv.Itoa(user.Id)))  //返回的是map[string]int
	goods := make([]map[string]interface{},len(goodsMap))
	i:= 0

	totalPrice := 0
	totalCount := 0

	for index,value := range goodsMap {
		skuid,_ := strconv.Atoi(index)
		var goodsSKU models.GoodsSKU
		goodsSKU.Id = skuid
		o.Read(&goodsSKU)

		temp := make(map[string]interface{})
		temp["goodsSKU"] = goodsSKU
		temp["count"] = value

		//计算单个商品的总价
		addCount := goodsSKU.Price * value
		temp["addCount"] = addCount

		//计算所有商品的总价和数量
		totalPrice += goodsSKU.Price * value
		totalCount += value

		goods[i] = temp
		i += 1
	}
	this.Data["goods"] = goods
	this.Data["totalPrice"] = totalPrice
	this.Data["totalCount"] = totalCount


	this.TplName = "cart.html"
}

//更新购物车数据
func (this *CartController)HandleUpdateMyCart()  {
	//1.获取数据
	skuid,err1 := this.GetInt("skuid")
	count,err2 := this.GetInt("count")
	resp := make(map[string]interface{})
	defer this.ServeJSON()

	//2.校验数据
	if err1 != nil || err2 != nil{
		resp["code"] = 1
		resp["errmsg"] = "请求数据不正确"
		this.Data["json"] = resp
		return
	}

	conn,err := redis.Dial("tcp","192.168.100.9:6379")
	if err != nil {

	}
	defer conn.Close()

	username := GetUserName(&this.Controller)
	if username == ""{
		resp["code"] = 3
		resp["errmsg"] = "当前用户未登录"
		this.Data["json"] = resp
		return
	}

	o := orm.NewOrm()
	var user models.User
	user.Name = username
	o.Read(&user,"Name")

	//3.处理数据
	conn.Do("hset","cart_"+strconv.Itoa(user.Id),skuid,count)

	//4.返回数据
	resp["code"] = 5
	resp["errmsg"] = "OK"
	this.Data["json"] = resp

}

//删除购物车数据
func (this *CartController)HandleDeleteMyCart()  {
	//1.获取数据
	skuid,err := this.GetInt("skuid")

	resp := make(map[string]interface{})
	defer this.ServeJSON()
	//2.校验数据
	if err != nil {
		resp["code"] = 1
		resp["errmsg"] = "请求数据不正确"
		this.Data["json"] = resp
		return
	}

	//3.处理数据
	conn,err := redis.Dial("tcp","192.168.100.9:6379")
	if err != nil {
		resp["code"] = 2
		resp["errmsg"] = "redis数据库连接失败"
		this.Data["json"] = resp
		return
	}
	defer conn.Close()

	username := GetUserName(&this.Controller)
	o := orm.NewOrm()
	var user models.User
	user.Name = username
	o.Read(&user,"Name")

	conn.Do("hdel","cart_"+strconv.Itoa(user.Id),skuid)

	//4.返回数据
	resp["code"] = 5
	resp["errmsg"] = "OK"
	this.Data["json"] = resp


}