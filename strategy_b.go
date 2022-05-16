package main

import (
	"github.com/jinzhu/copier"
	_ "github.com/jinzhu/gorm/dialects/mysql"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
	"strings"
)

/*
交易策略, 趋势战法
*/
type StrategyB struct {
	trading *Trading
}

//选股
func (bind *StrategyB) pickStocks(limit int) []StockDaily {
	//查找当日符合条件的股票
	var stocks []StockDaily
	if (bind.trading.Shanghai.Close.LessThan(bind.trading.Shanghai.Avp5)) ||
		//bind.trading.Shanghai.Low.LessThan(bind.trading.Shanghai.Avp10) ||
		//bind.trading.ThinkDays>0 ||
		//bind.trading.IsIndexLargeVolume ||
		false {
		return stocks
	}
	db := GetDB().Where("keep_limit_up_days = 0 and close<50 and pre_close<avp20 and close>avp20 and close>avp60 and close>avp120  and avp20>avp60 and avp60>avp120 and avp60_chg5>0 and date=?", bind.trading.Shanghai.Date.AddDate(0, 0, 0).Format("2006-01-02"))
	if err := db.Limit(limit).Find(&stocks).Error; err != nil {
		logrus.Error("查询出错, ", err)
		return stocks
	}

	var invest_stocks []StockDaily
	for _, stock := range stocks {
		if (strings.Contains(stock.Name, "退") || strings.Contains(stock.Name, "st") || strings.Contains(stock.Name, "ST")){
			continue
		}
		//var recent_daily []StockDaily
		//db = GetDB().Where("code=? and date < ? ", stock.Code,bind.trading.Shanghai.Date.Format("2006-01-02"), bind.trading.Shanghai.Date.Format("2006-01-02")).Order("date desc").Limit(20).Find(&recent_daily)
		//m := 0
		//for _, r := range recent_daily{
		//	if r.Turnover.LessThan(decimal.NewFromInt(2)){
		//		m = m + 1
		//	}
		//}
		//if m >= 10{
		//	continue
		//}
		invest_stocks = append(invest_stocks, stock)
	}
	return invest_stocks
}

//建仓策略
func (bind *StrategyB) OpenPosition() {
	bind.trading.MaxStockNum = 200
	inv_len := len(bind.trading.HoldPositionList)
	if inv_len < bind.trading.MaxStockNum {
		stocks := bind.pickStocks(int(bind.trading.LiquidAssets.Div(decimal.NewFromInt(3000)).IntPart()))
		if len(stocks) < 1 {
			return
		}
		//每只标的的投入资金
		//pos_size := bind.trading.LiquidAssets.Div(decimal.NewFromInt(int64(len(stocks))))
		//pos_size := bind.trading.TotalAssets.Div(decimal.NewFromInt(10))
		pos_size := decimal.NewFromInt(1)
		for _, StockDaily := range stocks {
			price := StockDaily.Close
			lots := bind.trading.AmountToShares(pos_size, price)
			if lots.LessThan(decimal.NewFromInt(1)) {
				lots = decimal.NewFromInt(1)
			}
			b := Daily{}
			copier.Copy(&b, &StockDaily)
			if err := bind.trading.IncreasePosition(b, price, decimal.NewFromInt(1)); err != nil {
				log.Error(err)
				return
			}
		}
	}
}

//加减仓策略
func (bind *StrategyB) DecisionPosition() {
	for _, inv := range bind.trading.GetHoldPositionList() {
		//当日开仓
		if (inv.LastOperation.action == Open && inv.LastOperation.date.Equal(bind.trading.Shanghai.Date)) ||
			(inv.StockDaily.Close.GreaterThanOrEqual(inv.StockDaily.PreClose.Mul(decimal.NewFromFloat(1.1)).Round(2))) ||
			false {
			continue
		}
		last_rate := inv.StockDaily.Close.Sub(inv.LastOperation.close).Div(inv.LastOperation.close)
		if //(inv.StockDaily.PctChg.GreaterThan(decimal.NewFromFloat(4)))||
		//大盘
		//(bind.trading.Shanghai.Close.LessThan(bind.trading.Shanghai.Avp5)) ||
		//(bind.trading.Shanghai.Low.LessThan(bind.trading.Shanghai.Avp10)) ||
		//大盘
		//bind.trading.ThinkDays>0 ||
			// 小于5日均线
			//(inv.StockDaily.Close.LessThan(inv.StockDaily.Avp5)) ||
			(inv.CumProfitRate.LessThanOrEqual(decimal.NewFromInt(0))) ||
			(inv.CumProfitRate.GreaterThan(decimal.NewFromFloat(300))) ||
			//(bind.trading.Shanghai.Date.Sub(inv.OpenDate).Hours() / 24 >=  20 && inv.CumProfitRate.LessThan(decimal.NewFromFloat(0.3))) ||
			// 换手大于20%
			//(inv.StockDaily.Turnover.GreaterThan(decimal.NewFromInt(20))) ||
			false {
			if err := bind.trading.DecreasePosition(inv.StockDaily, inv.StockDaily.Close, decimal.NewFromInt(int64(len(bind.trading.GetAvailableHold(inv))))); err != nil {
				log.Error(err)
				return
			}
		}else if last_rate.LessThanOrEqual(decimal.NewFromFloat(0.1)) && len(bind.trading.GetAvailableHold(inv)) > 1{
			if err := bind.trading.DecreasePosition(inv.StockDaily, inv.StockDaily.Close, decimal.NewFromInt(1)); err != nil {
				log.Error(err)
				return
			}
		}else if last_rate.GreaterThanOrEqual(decimal.NewFromFloat(0.1)){
			if err := bind.trading.IncreasePosition(inv.StockDaily, inv.StockDaily.Close, decimal.NewFromInt(1)); err != nil {
				log.Error(err)
				return
			}
		}
	}
}
