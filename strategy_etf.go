package main

import (
	"github.com/jinzhu/copier"
	_ "github.com/jinzhu/gorm/dialects/mysql"
	"github.com/shopspring/decimal"
	log "github.com/sirupsen/logrus"
	"sort"
)

/*
交易策略
*/
type StrategyEtf struct {
	trading *Trading
}

//选股
func (bind *StrategyEtf) pickStocks(limit int) []Daily {
	//查找当日符合条件的股票
	var stocks []EtfDaily
	var invest_stocks []Daily
	sub_query := GetDB().Model(&EtfDaily{}).Select("max(amount)").Where("date = ?", bind.trading.Shanghai.Date.Format("2006-01-02")).Group("top_members")
	GetDB().Model(&EtfDaily{}).Where("amount in (?) and avp20_chg5>0 and close>avp60 and pct_chg<0 and pre_pct_chg<0 and follow != 16  AND amount > 10000000 AND top_members !=''", sub_query, -2).Order("amount desc").Find(&stocks)

	//db := GetDB().Model(&EtfDaily{}).Where("code in('510210') and date = ?", bind.today_date).Find(&stocks)
	var investing_stock_follow []int64
	for _, v := range stocks {
		if (v.Follow > 0 && in_array(v.Follow, investing_stock_follow)) ||
			(in_array(v.Code, []string{
				"513500", "513100", "159941","513050","513600", "159920",
			})) {
			continue
		}
		for _, inv := range bind.trading.HoldPositionList{
			if inv.StockDaily.Code == v.Code{
				continue
			}
		}
		investing_stock_follow = append(investing_stock_follow, v.Follow)
		b := Daily{}
		copier.Copy(&b, &v)
		invest_stocks = append(invest_stocks, b)
	}
	return invest_stocks
	sort.Sort(DailyList(invest_stocks))
	if len(invest_stocks) < 2{
		return invest_stocks
	}
	s := []Daily{
		invest_stocks[len(invest_stocks)-1],
	}
	//for _, v := range invest_stocks[0 : int(len(invest_stocks)/2)]{
	//	if v.PctChg.GreaterThan(decimal.NewFromInt(0)){
	//		s = append(s, v)
	//		break
	//	}
	//}
	//for _, v := range invest_stocks[int(len(invest_stocks)/2) : ]{
	//	if v.PctChg.LessThan(decimal.NewFromInt(0)){
	//		s = append(s, v)
	//		break
	//	}
	//}
	return s
}

//建仓策略
func (bind *StrategyEtf) OpenPosition() {
	inv_len := len(bind.trading.HoldPositionList)
	if inv_len < bind.trading.MaxStockNum {
		stocks := bind.pickStocks(bind.trading.MaxStockNum - inv_len)
		if len(stocks) < 1 {
			return
		}
		//每只标的的投入资金
		pos_size := bind.trading.LiquidAssets.Div(decimal.NewFromInt(int64(len(stocks))))
		for _, StockDaily := range stocks {
			lots := bind.trading.AmountToShares(pos_size, StockDaily.Close)
			if lots.LessThan(decimal.NewFromInt(1)) {
				lots = decimal.NewFromInt(1)
			}
			if err := bind.trading.IncreasePosition(Daily(StockDaily), StockDaily.Close, lots); err != nil {
				log.Error(err)
				return
			}
		}
	}
}

//加减仓策略
func (bind *StrategyEtf) DecisionPosition() {
	for _, inv := range bind.trading.GetHoldPositionList() {
		if inv.LastOperation.action == Open && inv.LastOperation.date.Equal(bind.trading.Shanghai.Date) {
			continue
		}
		if (inv.StockDaily.Close.LessThan(inv.StockDaily.Avp60)) ||
			inv.StockDaily.Avp20Chg5.LessThan(decimal.NewFromInt(0)){
			if err := bind.trading.DecreasePosition(inv.StockDaily, inv.StockDaily.Close, decimal.NewFromInt(int64(len(bind.trading.GetAvailableHold(inv))))); err != nil {
				log.Error(err)
				return
			}
		}
	}
}
