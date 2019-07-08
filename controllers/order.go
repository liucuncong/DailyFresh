package controllers

import (
	"github.com/astaxie/beego"
	"strconv"
	"DailyFresh/models"
	"github.com/astaxie/beego/orm"
	"github.com/gomodule/redigo/redis"
	"time"
	"strings"
	"github.com/smartwalle/alipay"
	"fmt"
)

type OrderController struct {
	beego.Controller
}

//展示订单页面
func (this *OrderController)HandleOrder()  {

	//1.获取数据
	skuids:= this.GetStrings("skuid")

	//2.校验数据
	if len(skuids) == 0{
		beego.Info("购物车请求数据错误")
		this.Redirect("/user/myCart",302)
	}

	//3.处理数据
	o := orm.NewOrm()
	conn,err := redis.Dial("tcp","192.168.100.9:6379")
	if err != nil {
		beego.Info("redis数据库连接失败")
		return
	}
	defer conn.Close()

	//获取用户
	username := GetUserName(&this.Controller)
	var user models.User
	user.Name = username
	o.Read(&user,"Name")

	goodsBuffer := make([]map[string]interface{},len(skuids))

	totalPrice := 0
	totalCount := 0
	for index,skuid := range skuids {
		temp := make(map[string]interface{})

		id,_ := strconv.Atoi(skuid)
		//查询商品数据
		var goodsSKU models.GoodsSKU
		goodsSKU.Id = id
		o.Read(&goodsSKU)
		temp["goods"] = goodsSKU

		//查询商品数量
		count,_ := redis.Int(conn.Do("hget","cart_"+strconv.Itoa(user.Id),skuid))
		temp["count"] = count

		goodsBuffer[index] = temp

		// 计算小计
		amount := goodsSKU.Price * count
		temp["amount"] = amount

		//计算总金额和总件数
		totalCount += count
		totalPrice += amount

	}

	//获取地址数据
	var addr []models.Address
	o.QueryTable("Address").RelatedSel("User").Filter("User__Id",user.Id).All(&addr)
	this.Data["addr"] = addr

	//4.返回视图
	this.Data["username"] = username
	this.Data["goodsBuffer"] = goodsBuffer
	//传递总金额、总件数、运费及实付款
	this.Data["totalCount"] = totalCount
	this.Data["totalPrice"] = totalPrice
	transferPrice := 10
	this.Data["transferPrice"] = transferPrice
	this.Data["realyPrice"] = transferPrice + totalPrice
	//传递所有商品的id
	this.Data["skuids"] = skuids


	this.TplName = "place_order.html"
}

//添加订单
func (this *OrderController)HandleAddOrder()  {
	//1.获取数据
	addrId,err1 := this.GetInt("addrId")
	payId,err2 := this.GetInt("payId")
	skuid := this.GetString("skuids")
	ids := skuid[1:len(skuid)-1]
	skuids := strings.Split(ids," ")


	//totalPrice,err3 := this.GetInt("totalPrice")
	transferPrice,err4 := this.GetInt("transferPrice")
	realyPrice,err5 := this.GetInt("realyPrice")
	totalCount,err6 := this.GetInt("totalCount")

	resp := make(map[string]interface{})
	defer this.ServeJSON()
	//2.校验数据
	if err1 != nil || err2 != nil || err4 != nil || err5 != nil || err6 != nil || len(skuids) == 0{
		resp["code"] = 1
		resp["errmsg"] = "获取数据错误"
		this.Data["json"] = resp
		return
	}
	beego.Info(skuids)

	//3.处理数据（插入数据库）
	//向订单表插入数据
	o := orm.NewOrm()
	//标识事务开始
	o.Begin()

	username := GetUserName(&this.Controller)
	var user models.User
	user.Name = username
	o.Read(&user,"Name")

	var order models.OrderInfo
	order.OrderId = time.Now().Format("20060102150405")+strconv.Itoa(user.Id)
	order.User = &user
	order.Orderstatus = 1
	order.PayMethod = payId
	order.TotalCount = totalCount
	order.TotalPrice = realyPrice
	order.TransitPrice = transferPrice

	//查询地址
	var addr models.Address
	addr.Id = addrId
	o.Read(&addr)
	order.Address = &addr

	//执行插入操作
	o.Insert(&order)

	conn,_ := redis.Dial("tcp","192.168.100.9:6379")
	defer conn.Close()
	//向订单商品表插入数据
	for _,skuid := range skuids {
		id,_ := strconv.Atoi(skuid)

		var goods models.GoodsSKU
		goods.Id = id
		o.Read(&goods)
		i := 3
		for i > 0 {
			var orderGoods models.OrderGoods
			orderGoods.GoodsSKU = &goods
			orderGoods.OrderInfo = &order

			//从redis获取商品数量
			count,_ := redis.Int(conn.Do("hget","cart_"+strconv.Itoa(user.Id),skuid))
			if count > goods.Stock{
				resp["code"] = 2
				resp["errmsg"] = "商品库存不足"
				this.Data["json"] = resp
				//标识事务回滚
				o.Rollback()
				return
			}
			preCount := goods.Stock

			orderGoods.Count = count

			orderGoods.Price = count * goods.Price
			//插入
			o.Insert(&orderGoods)
			goods.Stock -= count
			goods.Sales += count

			updateCount,_ := o.QueryTable("GoodsSKU").Filter("Id",goods.Id).Filter("Stock",preCount).Update(orm.Params{"Stock":goods.Stock,"Sales":goods.Sales})
			if updateCount == 0 {
				if i > 0 {
					i -= 1
					continue
				}

				resp["code"] = 3
				resp["errmsg"] = "商品库存改变，订单提交失败"
				this.Data["json"] = resp
				//标识事务回滚
				o.Rollback()
				return

			} else {
				conn.Do("hdel","cart_"+strconv.Itoa(user.Id),goods.Id)
				break
			}
		}


	}

	//4.返回数据
	//提交事务
	o.Commit()
	resp["code"] = 5
	resp["errmsg"] = "OK"
	this.Data["json"] = resp

}

//处理支付
func (this *OrderController)HandlePay()  {
	var aliPublicKey =  `MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAzDRTAmaTayEVE5cPBGrLF1Q5EuOgyKNvr1fVIMT11b4ejVUCwLjh4qq5W0Xjfvryb2BwhTct/UtJkNSbTt8GdjONwJj3GRN4kweWtwkeUn0+OwtfSkpgdzHGEsKKw2cbNt6/drylhYxySz5hDy4JBgBIdTOY801OucgKMaplGupggBPIuEXSqbch8SOghKLpnPPCmnm2knctIvzzuLkvE+58N023cmXZSA0XNrE/HwQjV+wXbooo32xIY+FcWR/7KV8jA69twT+avCOxFo+qAe5cz7L9xcAFT71ipvUg4E55AXzOkuxiRhI1j5cOn6oWcyp8mec+bOhc3qBosyevfwIDAQAB` // 可选，支付宝提供给我们用于签名验证的公钥，通过支付宝管理后台获取
	var privateKey = `MIIEowIBAAKCAQEAzDRTAmaTayEVE5cPBGrLF1Q5EuOgyKNvr1fVIMT11b4ejVUC
wLjh4qq5W0Xjfvryb2BwhTct/UtJkNSbTt8GdjONwJj3GRN4kweWtwkeUn0+Owtf
SkpgdzHGEsKKw2cbNt6/drylhYxySz5hDy4JBgBIdTOY801OucgKMaplGupggBPI
uEXSqbch8SOghKLpnPPCmnm2knctIvzzuLkvE+58N023cmXZSA0XNrE/HwQjV+wX
booo32xIY+FcWR/7KV8jA69twT+avCOxFo+qAe5cz7L9xcAFT71ipvUg4E55AXzO
kuxiRhI1j5cOn6oWcyp8mec+bOhc3qBosyevfwIDAQABAoIBAGcsKKSV3vXJiTSU
pem9a08mJo/8oke9C7izz+L2oJ6VqCoQQYvN3ZMAXxZWgVKux76uIyurbXkEiO67
/Jwk4sbl1UDyCCaLR+hBdUyVNtTGoqKCZGrMmWCfrUvdLu77MSzP7jy3o4mOJFEP
+0oIIFb/3ZwZrbV/4b7L6xqc1Oh7iru5lDIomZPHnmQE5QkiYvtwjEqsxXNb31hL
u1BWGp1mtQ4QIujfX3bxpvu22K31KV+kgWzpwtLZB0Ao9M+2AnhAf23rYY/aVZaP
GJtal4UyIeB0Rh9xh4PfvLFddq6z42+SKOiNHtwWnKelLibNbUOSP//HV80J8Vtc
+dxz2AECgYEA+jC7hWFC9K3OXaEup75YTL1meHjYCqi4bDBh6kN32RHk/JwTmBC/
YN8R02AlsnB5s0ibHE4n3B544VHGZKVw4NKh32SHhFBWTe+r9hJYv6IdJq5y5boj
aznvZjCVwvBK4QVTx67tUCP6Q7G0VEFnNh2f+FHu9y6MB02WWk64l4ECgYEA0PI5
5RzFyqcaSt8N091tlqSkjCm5ytZgKcCbQaYbNo+Kj9YRmlUzXRXIoBPq84CaNv8Q
5ThrVQGOEzEnHR3MR5Cro6BPMOpbIAMLrw9yVPHVfH737NhHQYj59SfOM/h3u5WR
jQYXYfkbJ8CN4smrEDLokGNf8XwS/eITmkmbxv8CgYByF1MMSgQ8jB31eJFMEXM2
25AlFAaBJdukCpQ8PjQjGxPvVkVhLRH43QDGAaxvKPd2mH+Tctieeo7pQV9VelR1
UdhbhP5/ihsxQ0CJ4Gf0S7s7boYa2L1aIntXgIRq9yVOZB2Gi/DQgPeZcyom2gR1
GyFeHg75TZKxqeIMoKVxAQKBgH27BHOFmM+VNhEPn7Z5a9RWRl3BTfdsgHkfWU1r
srxmK67Z1cXUtw+waAVLdvoHzMSDP5tvE8cXJHMQBMVUhPQbbe0MLhr1KthcfM9e
sCHFU/2SOYXfryEUV7TZuw8y2HmcSvVdUPy3dUu6ZqatS653s9IOulEJpDP5smoJ
GR/pAoGBALFm3Y4BKVdbTeLmB2aXVWjZUx3SoGvXnV0+zGz2J78z+9TFaoADr4CN
1kXGXF77JphE77t/Bu+uvUif637QsUXOqK328ubxWHwoGSBlfIoVQpnnzjcwjUDp
xdVgmGgMnx+E0ny8ADAqvy+SP+UOO+J1rS2wMi5Hl9Ga7nrsJ7fx`// 必须，上一步中使用 RSA签名验签工具 生成的私钥


	appId := "2016092800614586"
	var client,err1 = alipay.New(appId, aliPublicKey, privateKey, false)
	// 将 key 的验证调整到初始化阶段
	if err1 != nil {
		fmt.Println(err1)
		return
	}

	//获取数据
	orderId := this.GetString("orderId")
	totalPrice := this.GetString("totalPrice")
	if orderId =="" || totalPrice == "" {
		beego.Info("订单号和付款总额不能为空")
		return
	}
	var p = alipay.TradeWapPay{}
	p.NotifyURL = "http://xxx"
	p.ReturnURL = "http://192.168.100.9:8086/user/payok"
	p.Subject = "天天生鲜购物平台"
	p.OutTradeNo = orderId
	p.TotalAmount = totalPrice
	p.ProductCode = "FAST_INSTANT_TRADE_PAY"

	var url, err = client.TradeWapPay(p)
	if err != nil {
		fmt.Println(err)
	}

	var payURL = url.String()
	this.Redirect(payURL,302)
	// 这个 payURL 即是用于支付的 URL，可将输出的内容复制，到浏览器中访问该 URL 即可打开支付页面。
}

//支付成功
func (this *OrderController)HandlePayOk()  {
	//获取数据
	//out_trade_no=9999999
	orderId := this.GetString("out_trade_no")

	//校验数据
	if orderId == ""{
		beego.Info("支付返回数据错误")
		this.Redirect("/user/userCenterOrder",302)
		return
	}

	//处理数据
	o := orm.NewOrm()
	count,_ := o.QueryTable("OrderInfo").Filter("OrderId",orderId).Update(orm.Params{"Orderstatus":2})
	if count == 0{
		beego.Info("更新数据失败")
		this.Redirect("/user/userCenterOrder",302)
		return
	}

	//返回视图
	this.Redirect("/user/userCenterOrder",302)
}

