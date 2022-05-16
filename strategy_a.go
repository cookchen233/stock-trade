package main

import (
	"github.com/jinzhu/copier"
	_ "github.com/jinzhu/gorm/dialects/mysql"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
)

/*
交易策略, 涨停板战法
*/
type StrategyA struct {
	trading *Trading
}

//选股
func (bind *StrategyA) pickStocks(limit int) []StockDaily {
	//查找当日符合条件的股票
	var stocks []StockDaily
	if //(bind.trading.Shanghai.Close.LessThan(bind.trading.Shanghai.Avp5) || bind.trading.Shanghai.Low.LessThan(bind.trading.Shanghai.Avp10)) ||
		//bind.trading.IsIndexLargeVolume ||
		false {
		return stocks
	}
	db := GetDB().Where("keep_limit_up_days>2 and (turnover<20 or pre_turnover < 20) and close<50 and date=?", bind.trading.Shanghai.Date.AddDate(0, 0, -1).Format("2006-01-02")).Order("rand()")
	if err := db.Limit(50).Find(&stocks).Error; err != nil {
		logrus.Error("查询出错, ", err)
		return stocks
	}

	var invest_stocks []StockDaily
	for _, stock := range stocks {
		//for _, investing_stock := range bind.trading.HoldPositionList {
		//	if investing_stock.StockDaily.Code == stock.Code {
		//		is_allow = false
		//		break
		//	}
		//}
		var recent_daily []StockDaily
		//最近10日的板
		db = GetDB().Where("code=? and date <= ? ", stock.Code,bind.trading.Shanghai.Date.Format("2006-01-02"), bind.trading.Shanghai.Date.Format("2006-01-02")).Order("date desc").Limit(21).Find(&recent_daily)
		is_allow := true
		for i, r := range recent_daily{
			if i > 1 && r.Close.GreaterThan(stock.Close){
				is_allow = false
				break
			}
		}
		//如果多于x个板
		//if len(recent_daily)-1 >= 3{
		//	continue
		//}
		//if recent_daily[1].Close.Sub(recent_daily[len(recent_daily)-1].Close).Div(recent_daily[len(recent_daily)-1].Close).GreaterThan(decimal.NewFromFloat(0.3)){
		//	continue
		//}
		//if recent_daily[0].Code == "002239"{
		//	pr("hehe", bind.trading.Shanghai.Date)
		//	pr("xxxd", recent_daily[1].Date, recent_daily[len(recent_daily)-1].Date, recent_daily[1].Close.Sub(recent_daily[len(recent_daily)-1].Close).Div(recent_daily[len(recent_daily)-1].Close))
		//	pr("xxxd", recent_daily[1].Close, recent_daily[len(recent_daily)-1].Close, recent_daily[1].Close.Sub(recent_daily[len(recent_daily)-1].Close).Div(recent_daily[len(recent_daily)-1].Close))
		//}
		//if recent_daily[0].Code == "000716"{
		//	pr("____________________________x________________________________")
		//	pr("fff", bind.trading.Shanghai.Date, recent_daily[0].Date, recent_daily[0].Open, recent_daily[0].Name)
		//}
		if is_allow{
			invest_stocks = append(invest_stocks, recent_daily[0])
			if len(invest_stocks) == limit{
				break
			}
		}
	}
	return invest_stocks
}

//建仓策略
func (bind *StrategyA) OpenPosition() {
	bind.trading.MaxStockNum = 10
	inv_len := len(bind.trading.HoldPositionList)
	if inv_len < bind.trading.MaxStockNum {
		stocks := bind.pickStocks(bind.trading.MaxStockNum - inv_len)
		if len(stocks) < 1 {
			return
		}
		//每只标的的投入资金
		//pos_size := bind.trading.LiquidAssets.Div(decimal.NewFromInt(int64(len(stocks))))
		pos_size := bind.trading.LiquidAssets.Div(decimal.NewFromInt(20))
		for _, StockDaily := range stocks {
			//当日换手小于3则无法买进
			if StockDaily.Turnover.LessThan(decimal.NewFromInt(3)) ||
				false {
				continue
			}
			limit_up_price := StockDaily.PreClose.Mul(decimal.NewFromFloat(1.1)).Round(2)
			price := limit_up_price
			//当日开盘小于涨停则买入价为开盘价
			if StockDaily.Open.LessThan(limit_up_price) {
				price = StockDaily.Open
			}
			lots := bind.trading.AmountToShares(pos_size, price)
			if lots.LessThan(decimal.NewFromInt(1)) {
				lots = decimal.NewFromInt(1)
			}
			b := Daily{}
			copier.Copy(&b, &StockDaily)
			if err := bind.trading.IncreasePosition(b, price, lots); err != nil {
				log.Error(err)
				return
			}
		}
	}
}

//加减仓策略
func (bind *StrategyA) DecisionPosition() {
	for _, inv := range bind.trading.GetHoldPositionList() {
		//当日开仓
		if (inv.LastOperation.action == Open && inv.LastOperation.date.Equal(bind.trading.Shanghai.Date)) ||
			//跌停
			(inv.StockDaily.Close.LessThanOrEqual(inv.StockDaily.PreClose.Mul(decimal.NewFromFloat(0.9)).Round(2))) ||
			false {
			continue
		}
		////昨日换手大于20%且今日未涨停
		//if (inv.StockDaily.PreTurnover.GreaterThan(decimal.NewFromInt(20)) && inv.StockDaily.IsLimitUp == 0) ||
		//	//未涨停
		//	(inv.StockDaily.IsLimitUp == 0) ||
		//	//未一字板且换手大于6%
		//	(inv.StockDaily.Amplitude.GreaterThan(decimal.NewFromInt(0)) && inv.StockDaily.Turnover.GreaterThan(decimal.NewFromInt(6))) ||
		//	//持仓超过7天且收盘价小于建仓以来的最高价
		//	(bind.trading.Shanghai.Date.Sub(inv.begin_date).Hours()/24 > 7 && inv.StockDaily.Close.LessThan(inv.high_close)) ||
		//	//连板指数放量
		//	(bind.trading.IsIndexLargeVolume) ||
		//	// 昨日和今日换手均大于20%
		//	(inv.StockDaily.PreTurnover.GreaterThan(decimal.NewFromInt(20)) && inv.StockDaily.Turnover.GreaterThan(decimal.NewFromInt(20))) {
		//	if err := bind.trading.dec_shares(inv, inv.shares); err != nil {
		//		log.Error(err)
		//		return
		//	}
		//}
		//盈利30%

		if //(inv.CumProfitRate.GreaterThan(decimal.NewFromFloat(30)))||
			//未涨停
			inv.StockDaily.Close.LessThan(inv.StockDaily.PreClose.Mul(decimal.NewFromFloat(1.1)).Round(2)) ||
			// 昨日和今日换手均大于20%
			(inv.StockDaily.PreTurnover.GreaterThan(decimal.NewFromInt(20)) && inv.StockDaily.Turnover.GreaterThan(decimal.NewFromInt(20))) ||
			false {
			if err := bind.trading.DecreasePosition(inv.StockDaily, inv.StockDaily.Close, decimal.NewFromInt(int64(len(bind.trading.GetAvailableHold(inv))))); err != nil {
				log.Error(err)
				return
			}
		}
	}
}
