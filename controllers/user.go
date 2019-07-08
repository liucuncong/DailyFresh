package controllers

import (
	"DailyFresh/models"
	"encoding/base64"
	"regexp"
	"strconv"

	"github.com/astaxie/beego"
	"github.com/astaxie/beego/orm"
	"github.com/astaxie/beego/utils"
	"github.com/gomodule/redigo/redis"
	"math"
)

type UserController struct {
	beego.Controller
}

// 显示注册页面
func (this *UserController) ShowReg() {
	this.TplName = "register.html"
}

// 处理注册数据
func (this *UserController) HandleReg() {
	//1.获取数据
	username := this.GetString("user_name")
	pwd := this.GetString("pwd")
	cpwd := this.GetString("cpwd")
	email := this.GetString("email")
	//2.校验数据
	if username == "" || pwd == "" || cpwd == "" || email == "" {
		this.Data["errmsg"] = "数据不完整，请重新注册~"
		this.TplName = "register.html"
		return
	}
	if pwd != cpwd {
		this.Data["errmsg"] = "两次注册密码不一致，请重新注册~"
		this.TplName = "register.html"
		return
	}
	reg, _ := regexp.Compile("^[A-Za-z0-9\u4e00-\u9fa5]+@[A-Za-z0-9_-]+(\\.[A-Za-z0-9_-]+)+$")
	// 匹配不上返回空，匹配上了返回匹配上的字符串
	res := reg.FindString(email)
	if res == "" {
		this.Data["errmsg"] = "邮箱格式不正确，请重新注册~"
		this.TplName = "register.html"
		return
	}

	//3.处理数据
	o := orm.NewOrm()

	var user models.User
	user.Name = username //在设计表的时候用户名设置的是全局唯一，所以不用查询判断是否用户名有重复的
	user.PassWord = pwd
	user.Email = email
	// 剩下两个字段有默认值

	_, err := o.Insert(&user)
	if err != nil {
		this.Data["errmsg"] = "注册失败，请更换数据注册~"
		this.TplName = "register.html"
		return
	}

	//发送邮件，激活用户
	emailConfig := `{"username":"1260034616@qq.com","password":"wdzlbolmnvnehjfa","host":"smtp.qq.com","port":587}`
	emailConn := utils.NewEMail(emailConfig)
	emailConn.From = "1260034616@qq.com"
	emailConn.To = []string{email}
	emailConn.Subject = "天天生鲜用户注册"
	// 注意：我们这里发送给用户的是激活请求地址（公网地址和端口号）
	emailConn.Text = "192.168.100.9:8086/active?id=" + strconv.Itoa(user.Id)
	emailConn.Send()

	//4.返回视图
	this.Ctx.WriteString("注册成功，请去相应邮箱激活用户")

}

// 激活处理
func (this *UserController) ActiveUser() {
	//1.获取数据
	id, err := this.GetInt("id")
	//2.校验数据
	if err != nil {
		this.Data["errmsg"] = "要激活的用户不存在"
		this.Redirect("/register", 302)
		return
	}

	//3.更新数据
	o := orm.NewOrm()
	var user models.User
	user.Id = id
	err = o.Read(&user)
	if err != nil {
		this.Data["errmsg"] = "要激活的用户不存在"
		this.Redirect("/register", 302)
		return
	}
	user.Active = true
	o.Update(&user)
	//4.返回视图
	this.Redirect("/login", 302)
}

//展示登陆页面
func (this *UserController) ShowLogin() {
	username := this.Ctx.GetCookie("username")
	//base64解码
	temp, _ := base64.StdEncoding.DecodeString(username)
	if string(temp) == "" {
		this.Data["username"] = ""
		this.Data["checked"] = ""
	} else {
		this.Data["username"] = string(temp)
		this.Data["checked"] = "checked"
	}

	this.TplName = "login.html"

}

//处理登陆业务
func (this *UserController) HandleLogin() {
	//1.获取数据
	username := this.GetString("username")
	pwd := this.GetString("pwd")

	//2.校验数据
	if username == "" || pwd == "" {
		this.Data["errmsg"] = "登陆数据不完整，请重新输入"
		this.TplName = "login.html"
		return
	}

	//3.处理数据
	o := orm.NewOrm()
	var user models.User
	user.Name = username

	err := o.Read(&user, "Name")
	if err != nil {
		this.Data["errmsg"] = "用户名或错误，请重新输入"
		this.TplName = "login.html"
		return
	}
	if user.PassWord != pwd {
		this.Data["errmsg"] = "或密码错误，请重新输入"
		this.TplName = "login.html"
		return
	}
	if user.Active != true {
		this.Data["errmsg"] = "用户未激活，请先激活邮箱"
		this.TplName = "login.html"
		return
	}

	//4.返回视图
	remember := this.GetString("remember")

	//base64加密
	if remember == "on" {
		temp := base64.StdEncoding.EncodeToString([]byte(username))
		this.Ctx.SetCookie("username", temp, 24*3600*30)
	} else {
		this.Ctx.SetCookie("username", username, -1)
	}

	this.SetSession("username", username)

	this.Redirect("/", 302)
}

//处理退出登陆业务
func (this *UserController) Logout() {
	this.DelSession("username")
	//跳转登陆
	this.Redirect("/login", 302)
}

//展示用户中心信息页
func (this *UserController) ShowUserCenterInfo() {
	username := GetUserName(&this.Controller)
	this.Data["username"] = username
	//思考:不登陆的时候能访问到这个函数吗

	//查询地址表的内容
	o := orm.NewOrm()
	//高级查询
	var addr models.Address
	o.QueryTable("Address").RelatedSel("User").Filter("User__Name", username).Filter("Isdefault", true).One(&addr)
	if addr.Id == 0 {
		this.Data["addr"] = ""
	} else {
		this.Data["addr"] = addr
	}

	//获取历史浏览记录
	conn, err := redis.Dial("tcp", "192.168.100.9:6379")
	if err != nil {
		beego.Info("redis连接错误")
	}
	defer conn.Close()

	//获取用户id
	var user models.User
	user.Name = username
	o.Read(&user, "Name")
	res, err := conn.Do("lrange", "history_"+strconv.Itoa(user.Id), 0, 4)
	//回复助手函数
	goodsIds, _ := redis.Ints(res, err)
	var goodsSKUs []models.GoodsSKU

	for _, value := range goodsIds {
		var goodsSKU models.GoodsSKU
		goodsSKU.Id = value
		o.Read(&goodsSKU)
		goodsSKUs = append(goodsSKUs, goodsSKU)
	}
	this.Data["goodsSKUs"] = goodsSKUs
	beego.Info(goodsSKUs)

	this.Layout = "userCenterLayout.html"
	this.TplName = "user_center_info.html"
}

//展示用户中心订单页
func (this *UserController) ShowUserCenterOrder() {
	userName := GetUserName(&this.Controller)
	o := orm.NewOrm()

	var user models.User
	user.Name = userName
	o.Read(&user,"Name")


	//分业实现
	//获取pageCount
	count,_ := o.QueryTable("OrderInfo").RelatedSel("User").Filter("User__Id",user.Id).Count()
	//设置每一页的订单数
	pageSize := 1
	//计算总页数
	pageCount := math.Ceil(float64(count)/float64(pageSize))
	//获取页码值
	pageIndex,err := this.GetInt("pageIndex")
	if err != nil {
		pageIndex = 1
	}
	//计算显示页码页
	pages := PageTool(int(pageCount),pageIndex)
	this.Data["pages"] = pages
	this.Data["pageIndex"] = pageIndex

	//获取上一页页码
	prePage := pageIndex -1
	if prePage <= 1{
		prePage = 1
	}
	this.Data["prePage"] = prePage

	//获取下一页页码
	nextPage := pageIndex + 1
	if nextPage > int(pageCount){
		nextPage = int(pageCount)
	}
	this.Data["nextPage"] = nextPage

	start := (pageIndex -1)*pageSize

	//获取订单表的数据
	var orderInfos []models.OrderInfo
	o.QueryTable("OrderInfo").RelatedSel("User").Filter("User__Id",user.Id).Limit(pageSize,start).All(&orderInfos)

	goodsBuffer := make([]map[string]interface{},len(orderInfos))

	for index,orderInfo := range orderInfos {
		temp := make(map[string]interface{})

		var orderGoods []models.OrderGoods
		o.QueryTable("OrderGoods").RelatedSel("OrderInfo","GoodsSKU").Filter("OrderInfo__Id",orderInfo.Id).All(&orderGoods)


		temp["orderInfo"] = orderInfo
		temp["orderGoods"] = orderGoods
		goodsBuffer[index] = temp

	}
	this.Data["goodsBuffer"] = goodsBuffer

	this.Layout = "userCenterLayout.html"
	this.TplName = "user_center_order.html"
}

//展示用户中心地址页
func (this *UserController) ShowUserCenterSite() {
	username := GetUserName(&this.Controller)
	//this.Data["username"] = username

	//获取地址信息
	o := orm.NewOrm()
	var addr models.Address
	o.QueryTable("Address").RelatedSel("User").Filter("User__Name", username).Filter("Isdefault", true).One(&addr)

	//传递给视图
	this.Data["addr"] = addr

	this.Layout = "userCenterLayout.html"
	this.TplName = "user_center_site.html"
}

//处理用户中心地址数据
func (this *UserController) HandleUserCenterSite() {
	//1.获取数据
	receiver := this.GetString("receiver")
	addr := this.GetString("addr")
	zipCode := this.GetString("zipCode")
	phone := this.GetString("phone")

	//2.校验数据
	if receiver == "" || addr == "" || zipCode == "" || phone == "" {
		beego.Info("添加数据不完整")
		this.Redirect("/user/userCenterSite", 302)
		return
	}

	// 手机号正则匹配
	reg, _ := regexp.Compile(`^(1[3|5|6|7|8][0-9]\d{8})$`)
	res := reg.FindString(phone)
	if res == "" {
		this.Data["errmsg"] = "手机号格式不正确，请改正后重试~"
		this.Redirect("/user/userCenterSite", 302)
		return
	}

	//3.处理数据
	o := orm.NewOrm()
	var addrUser models.Address
	addrUser.Isdefault = true
	err := o.Read(&addrUser, "Isdefault")
	//添加默认地址之前，需要把原来的默认地址更改成非默认地址
	if err == nil {
		addrUser.Isdefault = false
		o.Update(&addrUser)
	}

	//关联
	username := this.GetSession("username")
	var user models.User
	user.Name = username.(string)
	o.Read(&user, "Name")

	//更新默认地址时，给原来的地址对象的id赋值了，这时使用原来的地址对象插入，意思是用原来的ID做插入操作，会报错（id是主键，不允许重复）
	var addrUserNew models.Address
	addrUserNew.Receiver = receiver
	addrUserNew.Addr = addr
	addrUserNew.Phone = phone
	addrUserNew.Zipcode = zipCode
	addrUserNew.Isdefault = true
	addrUserNew.User = &user
	o.Insert(&addrUserNew)

	//4.返回视图
	this.Redirect("/user/userCenterSite", 302)

}
