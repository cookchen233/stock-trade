package main

import (
	"encoding/gob"
	"fmt"
	"github.com/jinzhu/copier"
	"github.com/patrickmn/go-cache"
	"github.com/shopspring/decimal"
	log "github.com/sirupsen/logrus"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func init() {
	cache_file := "./cache.gob"
	_, err := os.Lstat(cache_file)
	var M map[string]cache.Item
	if !os.IsNotExist(err) {
		File, _ := os.Open(cache_file)
		D := gob.NewDecoder(File)
		D.Decode(&M)
	}
	if len(M) > 0 {
		ca = cache.NewFrom(cache.NoExpiration, 10*time.Minute, M)
	} else {
		ca = cache.New(cache.NoExpiration, 10*time.Minute)
	}
}

func getAvpPrice(daily_list []Daily) decimal.Decimal {
	var sum decimal.Decimal
	len := decimal.NewFromInt(int64(len(daily_list)))
	for _, v := range daily_list {
		sum = sum.Add(v.Close)
	}
	return sum.Div(len)
}

func StockDailyListToDailyList(StockDaily_list []StockDaily) []Daily{
	var daily_list []Daily
	for _, v := range StockDaily_list{
		var daily Daily
		copier.Copy(&daily, &v)
		daily_list = append(daily_list, daily)
	}
	return daily_list
}
func IndexDailyListToDailyList(index_daily_list []IndexDaily) []Daily{
	var daily_list []Daily
	for _, v := range index_daily_list{
		var daily Daily
		copier.Copy(&daily, &v)
		daily_list = append(daily_list, daily)
	}
	return daily_list
}

func GetVolumeRatio(volume int64, daily_list []Daily) decimal.Decimal {
	var sum decimal.Decimal
	len := decimal.NewFromInt(int64(len(daily_list)))
	for _, v := range daily_list {
		sum = sum.Add(decimal.NewFromInt(v.Volume))
	}
	return decimal.NewFromInt(volume).Div(sum.Div(len))
}


func TestFixStockDaily(t *testing.T) {
	var stocks []StockDaily
	GetDB().Model(&StockDaily{}).Select("code").Group("code").Order("code asc").Find(&stocks)
	count := len(stocks)
	pr("总量 : ", count, "股票数", count)
	safe_go := NewSafeGo(100)
	var locker sync.Mutex
	for _, stock := range stocks {
		if _, found := ca.Get("fix_StockDaily_" + stock.Code); found {
			exec_lock(locker, func() {count--})
			continue
		}
		safe_go.Go(func(stock StockDaily) {
			defer func() {
				if err := recover(); err != nil {
					log.Error("发生错误", err)
					time.Sleep(time.Second * 600)
					main()
				}
			}()
			var daily_list []StockDaily
			db := GetDB().Where("code = ?", stock.Code).Order("date asc").Find(&daily_list)
			if err := db.Error; err != nil {
				log.Error("查询出错, ", err, stock.Code, stock.Name)
				return
			}
			for daily_i, daily := range daily_list {
				if daily_i> 0{
					//v.Avp10 = getAvpPrice(StockDailyListToDailyList(daily_list[max(i-10, 0):i]))
					daily.Avp5Chg = daily.Avp5.Sub(daily_list[daily_i-1].Avp5)
				}

				daily_list[daily_i] = daily
				if err := GetDB().Save(&daily).Error; err != nil {
					log.Error("更新失败, ", daily.Id, err)
				}
			}
			ca.Set("fix_StockDaily_" + daily_list[0].Code, 1, ne)
			exec_lock(locker, func() {
				count--
				pr("更新成功", daily_list[0].Name, daily_list[0].Code, "剩余", count)
			})
		}, stock)
	}
	safe_go.Wait()
	pr("全部完成")
}


func TestFixIndexDaily(t *testing.T) {
	var code_list []IndexDaily
	GetDB().Model(&IndexDaily{}).Select("code").Group("code").Find(&code_list)
	count := len(code_list)
	pr("总量 : ", count, "股票数", count)
	safe_go := NewSafeGo(200)
	var locker sync.Mutex
	for _, s := range code_list {
		safe_go.Go(func(s IndexDaily) {
			defer func() {
				if err := recover(); err != nil {
					log.Error("发生错误", err)
					time.Sleep(time.Second * 600)
					main()
				}
			}()
			var daily_list []IndexDaily
			db := GetDB().Where("code = ?", s.Code).Order("date asc").Find(&daily_list)
			if err := db.Error; err != nil {
				log.Error("查询出错, ", err, s.Code, s.Name)
				return
			}
			for i, v := range daily_list {
				if i> 0{
					v.Avp10 = getAvpPrice(IndexDailyListToDailyList(daily_list[max(i-10, 0):i]))
				}
				//if i > 5 {
				//	a := daily_list[max(i-5, 1) : len(daily_list)-1]
				//	b := []Daily{}
				//	copier.Copy(&b, &a)
				//	v.VolumeRatio = GetVolumeRatio(v.Volume, b)
				//}
				daily_list[i] = v
				if err := GetDB().Save(&v).Error; err != nil {
					log.Error("更新失败, ", v.Id, err)
				}
			}
			exec_lock(locker, func() {
				count--
				pr("更新成功", daily_list[0].Name, daily_list[0].Code, "剩余", count)
			})
		}, s)
	}
	safe_go.Wait()
	pr("全部完成")
}


func TestFixEtfDaily(t *testing.T) {
	var code_list []EtfDaily
	GetDB().Model(&EtfDaily{}).Select("code").Group("code").Find(&code_list)
	count := len(code_list)
	pr("总量 : ", count, "股票数", count)
	safe_go := NewSafeGo(200)
	var locker sync.Mutex
	for _, s := range code_list {
		safe_go.Go(func(s EtfDaily) {
			defer func() {
				if err := recover(); err != nil {
					log.Error("发生错误", err)
					time.Sleep(time.Second * 600)
					main()
				}
			}()
			var daily_list []EtfDaily
			db := GetDB().Where("code = ?", s.Code).Order("date asc").Find(&daily_list)
			if err := db.Error; err != nil {
				log.Error("查询出错, ", err, s.Code, s.Name)
				return
			}
			for i, v := range daily_list {
				if i  == 0 {
					if v.PreClose.IsZero(){
						v.PreClose = v.Close
					}
				}
				daily_list[i] = v
				if err := GetDB().Save(&v).Error; err != nil {
					log.Error("更新失败, ", v.Id, err)
				}
			}
			exec_lock(locker, func() {
				count--
				pr("更新成功", daily_list[0].Name, daily_list[0].Code, "剩余", count)
			})
		}, s)
	}
	safe_go.Wait()
	pr("全部完成")
}

func TestSnippet(t *testing.T) {
	var x string
	for i := 0; i <= 200; i++ {
		x += "f" + strconv.Itoa(i) + ","
	}
	fmt.Println(x)
}

func TestFt(t *testing.T) {
	var y float64
	f := 0.2/10000
	p := 0.5/100
	rand.Seed(time.Now().UnixNano())
	for j := 0; j <= 10000; j++ {
		x := 1.0
		for i := 0; i <= 10*100; i++ {
			if rand.Intn(100) < 50 {
				if rand.Intn(100) < 60 {
					x = x * (1 - f) * (1 + p) * (1 - f)
				} else {
					x = x * (1 - f) * (1 - p) * (1 - f)
				}
			}
		}
		y = y + x
	}
	pr(fmt.Sprintf("%.4f", y/10000))
}


func TestEastMoneyFields(t *testing.T) {
	f := []string{}
	for j := 0; j <= 1000; j++ {
		f = append(f, fmt.Sprintf("f%v", j))
	}
	pr(strings.Join(f, ","))
}

func TestHello(t *testing.T){
	type s[]string

	s3 := make(s, 1)
	s3[0] = "d"
	s3  = append(s3, "e")
	s3  = append(s3, "f")
	s3  = append(s3, "g")
	s3  = append(s3, "h")
	fmt.Println(len(s3), cap(s3))
}


