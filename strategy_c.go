package main

import (
	"github.com/jinzhu/copier"
	_ "github.com/jinzhu/gorm/dialects/mysql"
	"github.com/shopspring/decimal"
	log "github.com/sirupsen/logrus"
	"strings"
)

/*
交易策略, 反包溢价
*/
type StrategyC struct {
	trading *Trading
}

//选股
func (bind *StrategyC) pickStocks(limit int) []StockDaily {
	//查找当日符合条件的股票
	var stocks []StockDaily
	if bind.trading.IsIndexLargeVolume {
		//return stocks
	}
	GetDB().Where("turnover > pre_turnover and volume_ratio > 2 and pct_chg>3 and pct_chg<11 and keep_limit_up_days = 0 and date=?", bind.trading.Shanghai.Date.Format("2006-01-02")).Limit(100).Order("rand()").Find(&stocks)
	var invest_stocks []StockDaily
	for _, stock := range stocks{
		if (strings.Contains(stock.Name, "退") || strings.Contains(stock.Name, "st") || strings.Contains(stock.Name, "ST")){
			continue
		}
		if stock.Code[ : 3] == "300" || stock.Code[ : 3] == "688"{
			continue
		}
		if stock.PctChg.Sub(stock.PrePctChg).LessThan(decimal.NewFromInt(1)) {
			continue
		}
		if stock.High.Sub(stock.Close).Div(stock.Close).GreaterThan(decimal.NewFromFloat(0.01)){
			continue
		}
		var recent_daily []StockDaily
		GetDB().Where("code=? and date < ? ", stock.Code,bind.trading.Shanghai.Date.Format("2006-01-02"), bind.trading.Shanghai.Date.AddDate(0, 0, -1).Format("2006-01-02")).Order("date desc").Limit(10).Find(&recent_daily)
		m := 0
		for _, r := range recent_daily{
			if r.Turnover.GreaterThan(decimal.NewFromInt(20)){
				m = m + 1
			}
		}
		if m >= 2{
			continue
		}

		invest_stocks = append(invest_stocks, stock)
		if(len(invest_stocks) == limit){
			return invest_stocks
		}
	}
	return invest_stocks
}

//建仓策略
func (bind *StrategyC) OpenPosition() {
	bind.trading.MaxStockNum = 20
	inv_len := len(bind.trading.HoldPositionList)
	if inv_len < bind.trading.MaxStockNum {
		stocks := bind.pickStocks(bind.trading.MaxStockNum - inv_len)
		if len(stocks) < 1 {
			return
		}
		//每只标的的投入资金
		//pos_size := bind.trading.LiquidAssets.Div(decimal.NewFromInt(int64(len(stocks))))
		pos_size := bind.trading.TotalAssets.Div(decimal.NewFromInt(int64(len(stocks))))
		for _, StockDaily := range stocks {
			price := StockDaily.Close
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
func (bind *StrategyC) DecisionPosition() {
	for _, inv := range bind.trading.GetHoldPositionList() {
		if (inv.LastOperation.action == Open && inv.LastOperation.date.Equal(bind.trading.Shanghai.Date)) ||
			//(inv.StockDaily.Close.GreaterThanOrEqual(inv.StockDaily.PreClose.Mul(decimal.NewFromFloat(1.1)).Round(2))) ||
			false {
			continue
		}
		SellPrice := inv.StockDaily.Close
		//pct_3_price := inv.StockDaily.PreClose.Mul(decimal.NewFromFloat(1.03))
		pct_1_price := inv.StockDaily.PreClose.Mul(decimal.NewFromFloat(1.01))
		if inv.StockDaily.Open.GreaterThan(pct_1_price){
			SellPrice = inv.StockDaily.Open
		}
		if err := bind.trading.DecreasePosition(inv.StockDaily, SellPrice, decimal.NewFromInt(int64(len(bind.trading.GetHold(inv))))); err != nil {
			log.Error(err)
			return
		}
	}
}
