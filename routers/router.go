package routers

import (
	"DailyFresh/controllers"
	"github.com/astaxie/beego"
	"github.com/astaxie/beego/context"
)

func init() {
	beego.InsertFilter("/user/*",beego.BeforeExec,filterFunc)

    //beego.Router("/", &controllers.MainController{})
    beego.Router("/register",&controllers.UserController{},"get:ShowReg;post:HandleReg")
    //激活用户
    beego.Router("/active",&controllers.UserController{},"get:ActiveUser")
    //用户登录
    beego.Router("/login",&controllers.UserController{},"get:ShowLogin;post:HandleLogin")
	//跳转首页
	beego.Router("/",&controllers.GoodsController{},"get:ShowIndex")
	//退出登陆
	beego.Router("/user/logout",&controllers.UserController{},"get:Logout")
	//用户中心信息页
	beego.Router("/user/userCenterInfo",&controllers.UserController{},"get:ShowUserCenterInfo")
	//用户中心订单页
	beego.Router("/user/userCenterOrder",&controllers.UserController{},"get:ShowUserCenterOrder")
	//用户中心地址页
	beego.Router("/user/userCenterSite",&controllers.UserController{},"get:ShowUserCenterSite;post:HandleUserCenterSite")
	//商品详情展示
	beego.Router("/goodsDetail",&controllers.GoodsController{},"get:ShowGoodsDetail")
	//商品列表页
	beego.Router("/goodsList",&controllers.GoodsController{},"get:ShowList")
	//商品搜索
	beego.Router("/goodsSearch",&controllers.GoodsController{},"post:HandleSearch")
	//添加购物车
	beego.Router("/user/addCart",&controllers.CartController{},"post:HandleAddCart")
	//展示购物车页面
	beego.Router("/user/myCart",&controllers.CartController{},"get:ShowMyCart")
	//更新购物车数量
	beego.Router("/user/updateCart",&controllers.CartController{},"post:HandleUpdateMyCart")
	//删除购物车数据
	beego.Router("/user/deleteCart",&controllers.CartController{},"post:HandleDeleteMyCart")
	//展示订单页面
	beego.Router("/user/showOrder",&controllers.OrderController{},"post:HandleOrder")
	//添加订单
	beego.Router("/user/addOrder",&controllers.OrderController{},"post:HandleAddOrder")
	//处理支付
	beego.Router("/user/pay",&controllers.OrderController{},"get:HandlePay")
	//支付成功
	beego.Router("/user/payok",&controllers.OrderController{},"get:HandlePayOk")

}

var filterFunc = func(ctx *context.Context) {
	username := ctx.Input.Session("username")
	if username == nil{
		ctx.Redirect(302,"/login")
		return
	}
}
