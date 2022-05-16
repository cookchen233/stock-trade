package main

import (
	"github.com/jinzhu/copier"
	_ "github.com/jinzhu/gorm/dialects/mysql"
	"github.com/shopspring/decimal"
	log "github.com/sirupsen/logrus"
)

/*
交易策略
*/
type StrategyEtfB struct {
	trading *Trading
}

//选股
func (bind *StrategyEtfB) pickStocks(limit int) []EtfDaily {
	//查找当日符合条件的股票
	var stocks []EtfDaily
	if (bind.trading.Shanghai.Close.LessThan(bind.trading.Shanghai.Avp5)) ||
		//bind.trading.Shanghai.Low.LessThan(bind.trading.Shanghai.Avp10) ||
		//bind.trading.ThinkDays>0 ||
		//bind.trading.IsIndexLargeVolume ||
		false {
		return stocks
	}
	GetDB().Model(&EtfDaily{}).Where("code = 510760 and date = ?", bind.trading.Shanghai.Date).Limit(limit).Find(&stocks)
	return stocks
}

//建仓策略
func (bind *StrategyEtfB) OpenPosition() {
	bind.trading.MaxStockNum = 1
	inv_len := len(bind.trading.HoldPositionList)
	if inv_len < bind.trading.MaxStockNum {
		stocks := bind.pickStocks(bind.trading.MaxStockNum - inv_len)
		if len(stocks) < 1 {
			return
		}
		//每只标的的投入资金
		//pos_size := bind.trading.LiquidAssets.Div(decimal.NewFromInt(int64(len(stocks))))
		pos_size := bind.trading.LiquidAssets.Div(decimal.NewFromInt(2))
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
func (bind *StrategyEtfB) DecisionPosition() {
	for _, inv := range bind.trading.GetHoldPositionList() {
		if (inv.LastOperation.action == Open && inv.LastOperation.date.Equal(bind.trading.Shanghai.Date)) ||
			(inv.StockDaily.Close.GreaterThanOrEqual(inv.StockDaily.PreClose.Mul(decimal.NewFromFloat(1.1)).Round(2))) ||
			false {
			continue
		}
		if //(inv.CumProfitRate.GreaterThan(decimal.NewFromFloat(30)))||
		(bind.trading.Shanghai.Close.LessThan(bind.trading.Shanghai.Avp5)) ||
			(bind.trading.Shanghai.Low.LessThan(bind.trading.Shanghai.Avp10)) ||
			false {
			if err := bind.trading.DecreasePosition(inv.StockDaily, inv.StockDaily.Close, decimal.NewFromInt(int64(len(bind.trading.GetAvailableHold(inv))))); err != nil {
				log.Error(err)
				return
			}
		}
	}
}
