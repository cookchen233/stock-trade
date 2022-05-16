package main

import (
	"fmt"
	"github.com/jinzhu/copier"
	_ "github.com/jinzhu/gorm/dialects/mysql"
	"github.com/shopspring/decimal"
	log "github.com/sirupsen/logrus"
	"github.com/xuri/excelize"
	"os"
	"reflect"
	"sort"
	"strconv"
	"sync"
	"time"
)

//股票开平仓详细
type HoldLotsInfo struct {
	BuyDate   time.Time       //买入日期
	SellDate  time.Time       //卖出日期
	BuyPrice  decimal.Decimal //买入价格
	SellPrice decimal.Decimal //卖出价格
}

//最新操作
type LastOperation struct {
	date   time.Time //操作时间
	action Action    //操作类别
	close decimal.Decimal    //操作时收盘价
	note   string    //备注
}

//当前持仓股票
type Position struct {
	HoldLotsInfoList []HoldLotsInfo
	StockDaily         Daily
	CreateTime         time.Time
	OpenDate           time.Time       //建仓日期
	CostPrice          decimal.Decimal //成本价
	TodayInvestCosts  decimal.Decimal //净投入金额
	InvestCosts        decimal.Decimal //净投入金额
	HoldAssets         decimal.Decimal //持有市值
	TodayProfit        decimal.Decimal //当日收益
	TodayProfitRate   decimal.Decimal //当日收益率
	CumProfit          decimal.Decimal //累计收益
	CumProfitRate     decimal.Decimal //累计收益率
	LastOperation      LastOperation   //最新操作
	StockHighPrice    decimal.Decimal //建仓以来最高价
}

type PositionList []Position

func (a PositionList) Len() int           { return len(a) }
func (a PositionList) Less(i, j int) bool { return a[i].CreateTime.Before(a[j].CreateTime) }
func (a PositionList) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }


type DailyList []Daily

func (a DailyList) Len() int           { return len(a) }
func (a DailyList) Less(i, j int) bool { return a[i].PctChg.LessThan(a[j].PctChg) }
func (a DailyList) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }

type StrategyInterface interface {
	OpenPosition()
	DecisionPosition()
}

type Trading struct {
	mu                             sync.Mutex
	StockBeginInvest             decimal.Decimal   //每只股票最大初始投入资金
	MaxStockNum                  int               //最多持股数
	OriginalAssets                decimal.Decimal   //原始资产
	TotalAssets                   decimal.Decimal   //总资产
	LiquidAssets                  decimal.Decimal   //可用资金
	HoldAssets                    decimal.Decimal   //持仓市值
	HoldProfit                    decimal.Decimal   //持仓收益
	HoldProfitRate               decimal.Decimal   //持仓收益率
	InvestCosts                   decimal.Decimal   //净投入
	TodayInvestCosts             decimal.Decimal   //当日净投入
	InvestCostsRate              decimal.Decimal   //净投入收益率
	TodayProfit                   decimal.Decimal   //当日收益
	TodayProfitRate              decimal.Decimal   //当日收益率
	CumProfit                     decimal.Decimal   //累计收益
	HighInvestCosts              decimal.Decimal   //最高净投入
	HighInvestCostsRate         decimal.Decimal   //最高净投入收益率
	dayInvestList                []decimal.Decimal //累计净投入记录
	HoldPositionList             []Position
	TodayClosePostionList       []Position
	ClosedPostionList            []Position
	StartDate                     string
	EndDate                       string
	OperationType                 []string
	xlsx                           *excelize.File
	StockPortfolio                [][]string
	Shanghai                       IndexDaily
	strategy                       StrategyInterface
	RiskyKeepLimitUpIndexDays []time.Time
	IsIndexLargeVolume          bool
	IsRiskyDay                   bool
	BuyTax                        decimal.Decimal //买入税费
	SellTax                       decimal.Decimal //卖出税费
	ThinkRate decimal.Decimal
	ThinkDays int
}

type Action int

func (a Action) String() string {
	switch a {
	case Open:
		return "建仓"
	case Increase:
		return "加仓"
	case Decrease:
		return "减仓"
	case Close:
		return "清仓"
	default:
		return ""
	}
}

const (
	Open Action = iota
	Increase
	Decrease
	Close
)

func NewTrading(OriginalAssets decimal.Decimal, StartDate string, EndDate string) Trading {
	trading := Trading{
		OriginalAssets: OriginalAssets,
		LiquidAssets:   OriginalAssets,
		StartDate:      StartDate,
		EndDate:        EndDate,
		MaxStockNum:   20,
		BuyTax:         decimal.NewFromFloat(0.00012),
		SellTax:        decimal.NewFromFloat(0.00112),
		ThinkRate:        decimal.NewFromFloat(20),
		ThinkDays:        0,
	}
	return trading

}

func (bind *Trading) SetStrategy(strategy StrategyInterface) {
	bind.strategy = strategy
}

//是否在危险日期
func (bind *Trading) isDayInRiskyDays(date time.Time) bool {
	risky_days := [][]string{
		{"2018-01-24", "2018-01-30"},
		{"2018-02-08", "2018-02-23"},
		{"2018-03-14", "2018-03-20"},
		{"2018-04-24", "2018-04-30"},
		{"2018-04-25", "2018-05-01"},
		{"2018-06-06", "2018-06-12"},
		{"2018-07-25", "2018-07-31"},
		{"2018-09-19", "2018-10-08"},
		{"2018-10-31", "2018-11-05"},
		{"2018-12-12", "2018-12-18"},

		{"2019-01-23", "2019-02-11"},
		{"2019-03-13", "2019-03-19"},
		{"2019-04-24", "2019-04-30"},
		{"2019-06-12", "2019-06-18"},
		{"2019-07-24", "2019-07-30"},
		{"2019-09-11", "2019-09-17"},
		{"2019-09-25", "2019-10-08"},
		{"2019-12-04", "2019-12-10"},

		{"2020-01-16", "2020-02-03"},
		{"2020-03-11", "2020-03-17"}, //二
		{"2020-04-22", "2020-04-28"}, //二
		{"2020-06-03", "2020-06-09"}, //二
		{"2020-07-22", "2020-07-28"}, //二
		{"2020-09-09", "2020-09-15"}, //二
		{"2020-09-24", "2020-10-08"},
		{"2020-10-27", "2020-11-02"}, //三
		{"2020-12-09", "2020-12-15"}, //二

		{"2021-01-21", "2021-02-03"},
		{"2021-03-10", "2021-03-16"},
		{"2021-03-31", "2021-04-06"},
		{"2021-05-12", "2021-05-18"},
		{"2021-06-09", "2021-06-15"},
		{"2021-07-21", "2021-07-27"},
		{"2021-08-11", "2021-08-17"},
		{"2021-09-15", "2021-10-12"},
		{"2021-10-27", "2021-11-02"},
		{"2021-11-17", "2021-11-23"},
		{"2021-12-08", "2021-12-14"},
	}
	for _, v := range risky_days {
		mint, _ := time.ParseInLocation("2006-01-02", v[0], loc)
		maxt, _ := time.ParseInLocation("2006-01-02", v[1], loc)
		if date.After(mint) && date.Before(maxt) {
			return true
		}
	}
	return false
}

//资金可购买手数
func (bind *Trading) AmountToShares(amount decimal.Decimal, price decimal.Decimal) decimal.Decimal {
	return amount.Div(price).Div(decimal.NewFromInt(100)).Floor()
}

//获取股票当天 k 线信息
func (bind *Trading) GetTodayStockDaily(StockDaily StockDaily) StockDaily {
	if StockDaily.Date.Before(bind.Shanghai.Date) {
		if _, ok := interface{}(StockDaily).(StockDaily); ok{

		}
		var today_StockDaily StockDaily
		GetDB().Where("date = ? AND code = ?", bind.Shanghai.Date.Format("2006-01-02"), StockDaily.Code).First(&today_StockDaily)
		if today_StockDaily.Id > 0 {
			StockDaily = today_StockDaily
		}
	}
	return StockDaily
}
//更新股票当天 k 线信息
func (bind *Trading) GetDailyByDate(StockDaily Daily, date time.Time) Daily{
	if StockDaily.Date.Before(date) {
		if _, ok := interface{}(StockDaily).(StockDaily); !ok{
			var new_StockDaily StockDaily
			GetDB().Where("date = ? AND code = ?", date.Format("2006-01-02"), StockDaily.Code).First(&new_StockDaily)
			if new_StockDaily.Id > 0 {
				b := Daily{}
				copier.Copy(&b, &new_StockDaily)
				return b
			}
		}else{
			var new_StockDaily EtfDaily
			GetDB().Where("date = ? AND code = ?", date.Format("2006-01-02"), StockDaily.Code).First(&new_StockDaily)
			if new_StockDaily.Id > 0 {
				b := Daily{}
				copier.Copy(&b, &new_StockDaily)
				return b
			}
		}

	}
	return StockDaily
}

//提取可用仓位
func (bind *Trading) GetAvailableHold(stock Position) []HoldLotsInfo {
	var hold_list []HoldLotsInfo
	for _, hold := range stock.HoldLotsInfoList {
		if hold.BuyDate.Before(bind.Shanghai.Date) && hold.SellDate.IsZero() {
			hold_list = append(hold_list, hold)
		}
	}
	return hold_list
}

//提取持股仓位
func (bind *Trading) GetHold(stock Position) []HoldLotsInfo {
	var hold_list []HoldLotsInfo
	for _, hold := range stock.HoldLotsInfoList {
		if hold.SellDate.IsZero() {
			hold_list = append(hold_list, hold)
		}
	}
	return hold_list
}

//加仓/建仓
func (bind *Trading) IncreasePosition(StockDaily Daily, price decimal.Decimal, lots decimal.Decimal) error {
	//需要资金
	costs := price.Mul(lots.Mul(decimal.NewFromInt(100)))
	if bind.LiquidAssets.LessThan(costs) {
		//return nil
		return fmt.Errorf("当前准备金不足, 剩余%v, 需要%v [%v, %v]", bind.LiquidAssets, costs, StockDaily.Name, bind.Shanghai.Date)
	}
	exec_lock(bind.mu, func() {
		//定义操作信息
		LastOperation := LastOperation{
			date:   bind.Shanghai.Date,
			action: Open, //建仓
			close : StockDaily.Close,
		}
		var inv_list []Position
		//初始化持仓信息
		investing_stock := Position{
			HoldLotsInfoList: []HoldLotsInfo{},
			StockDaily:         StockDaily,
			OpenDate:           bind.Shanghai.Date,
			CreateTime:         time.Now(),
		}
		for _, inv := range bind.HoldPositionList {
			if inv.StockDaily.Code != StockDaily.Code {
				inv_list = append(inv_list, inv)
			} else {
				//已经存在持仓
				investing_stock = inv
				LastOperation.action = Increase //加仓
			}
		}
		LastOperation.note = fmt.Sprintf("%v->%v", len(bind.GetAvailableHold(investing_stock))*100, decimal.NewFromInt(int64(len(bind.GetAvailableHold(investing_stock)))).Add(lots).Mul(decimal.NewFromInt(100)))
		//更新该股票仓位信息
		for i := 0; decimal.NewFromInt(int64(i)).LessThan(lots); i++ {
			investing_stock.HoldLotsInfoList = append(investing_stock.HoldLotsInfoList, HoldLotsInfo{
				BuyDate:  bind.Shanghai.Date,
				BuyPrice: price,
			})
		}
		investing_stock.LastOperation = LastOperation
		//更新持仓列表
		bind.HoldPositionList = append(inv_list, investing_stock)
		//减少可用资金
		bind.LiquidAssets = bind.LiquidAssets.Sub(costs.Mul(decimal.NewFromInt(1).Add(bind.BuyTax)))
		log.Debugf("%v %v, %v, %v", LastOperation.action, LastOperation.note, StockDaily.Name, bind.Shanghai.Date)
	})
	return nil
}

//减仓/清仓
func (bind *Trading) DecreasePosition(StockDaily Daily, price decimal.Decimal, lots decimal.Decimal) error {
	var inv_list []Position
	var investing_stock Position
	for _, inv := range bind.HoldPositionList {
		if inv.StockDaily.Code != StockDaily.Code {
			inv_list = append(inv_list, inv)
		} else {
			investing_stock = inv
		}
	}
	//可用仓位(非今日加仓仓位)
	hold_len := len(bind.GetHold(investing_stock))
	if decimal.NewFromInt(int64(len(bind.GetAvailableHold(investing_stock)))).LessThan(lots) {
		return fmt.Errorf("可用持仓不足, 持仓%v, 可用%v, 减仓%v [%v, %v]", hold_len*100, len(bind.GetAvailableHold(investing_stock))*100, lots, StockDaily.Name, bind.Shanghai.Date)
	}
	exec_lock(bind.mu, func() {

		//减仓
		var hold_list []HoldLotsInfo
		var i decimal.Decimal
		for _, hold := range investing_stock.HoldLotsInfoList {
			if i.LessThan(lots) && hold.BuyDate.Before(bind.Shanghai.Date) && hold.SellDate.IsZero() {
				i = i.Add(decimal.NewFromInt(1))
				hold.SellDate = bind.Shanghai.Date
				hold.SellPrice = price
			}
			hold_list = append(hold_list, hold)
		}
		//更新该股票仓位信息
		investing_stock.HoldLotsInfoList = hold_list
		//更新操作信息
		LastOperation := LastOperation{
			date:   bind.Shanghai.Date,
			action: Decrease,
			close : StockDaily.Close,
			note:   fmt.Sprintf("%v->%v", hold_len * 100, (int64(hold_len) - lots.IntPart())*100),
		}
		investing_stock.LastOperation = LastOperation
		//清仓
		if len(bind.GetHold(investing_stock)) < 1 {
			LastOperation.action = Close
			investing_stock.LastOperation = LastOperation
			//放进当日清仓列表
			bind.TodayClosePostionList = append(bind.TodayClosePostionList, investing_stock)
		} else {
			inv_list = append(inv_list, investing_stock)
		}
		//更新持仓列表
		bind.HoldPositionList = inv_list
		//增加可用资金
		income := price.Mul(lots.Mul(decimal.NewFromInt(100))).Mul(decimal.NewFromInt(1).Sub(bind.SellTax))
		bind.LiquidAssets = bind.LiquidAssets.Add(income)
		log.Debugf("%v %v, %v, %v", LastOperation.action, LastOperation.note, StockDaily.Name, bind.Shanghai.Date)
	})
	return nil
}

//更新持仓收益
func (bind *Trading) ComputeInvestingStock() {
	bind.InvestCosts = decimal.NewFromInt(0)
	bind.TodayInvestCosts = decimal.NewFromInt(0)
	bind.HoldAssets = decimal.NewFromInt(0)
	bind.TodayProfit = decimal.NewFromInt(0)
	bind.CumProfit = decimal.NewFromInt(0)
	bind.TotalAssets = decimal.NewFromInt(0)
	bind.HoldProfit = decimal.NewFromInt(0)
	//计算正在持仓
	var inv_list []Position
	for _, inv := range bind.HoldPositionList {
		//inv.StockDaily = bind.GetTodayStockDaily(inv.StockDaily)
		inv.StockDaily = bind.GetDailyByDate(inv.StockDaily, bind.Shanghai.Date)
		inv.TodayProfit = decimal.NewFromInt(0)
		inv.TodayInvestCosts = decimal.NewFromInt(0)
		inv.InvestCosts = decimal.NewFromInt(0)
		inv.HoldAssets = decimal.NewFromInt(0)
		inv.CumProfit = decimal.NewFromInt(0)
		if inv.StockDaily.Close.GreaterThan(inv.StockHighPrice) {
			inv.StockHighPrice = inv.StockDaily.Close
		}
		for _, hold := range inv.HoldLotsInfoList {
			//计算累计收益
			//持有中
			if hold.SellDate.IsZero() {
				//计算当日收益
				//之前买入的收益
				if hold.BuyDate.Before(bind.Shanghai.Date) {
					inv.TodayProfit = inv.TodayProfit.Add(inv.StockDaily.Close.Sub(inv.StockDaily.PreClose).Mul(decimal.NewFromInt(100)))
				} else { //当天买入的收益
					inv.TodayProfit = inv.TodayProfit.Add(inv.StockDaily.Close.Sub(hold.BuyPrice).Mul(decimal.NewFromInt(100)))
				}
				//计算当日成本
				//之前买入的
				if hold.BuyDate.Before(bind.Shanghai.Date) {
					inv.TodayInvestCosts = inv.TodayInvestCosts.Add(inv.StockDaily.PreClose.Mul(decimal.NewFromInt(100)))
				} else { //当天买入的
					inv.TodayInvestCosts = inv.TodayInvestCosts.Add(hold.BuyPrice.Mul(decimal.NewFromInt(100)))
				}
				inv.CumProfit = inv.CumProfit.Add(inv.StockDaily.Close.Sub(hold.BuyPrice).Mul(decimal.NewFromInt(100)))
				inv.InvestCosts = inv.InvestCosts.Add(hold.BuyPrice.Mul(decimal.NewFromInt(100)))
				inv.HoldAssets = inv.HoldAssets.Add(inv.StockDaily.Close.Mul(decimal.NewFromInt(100)))
				//今天卖出的
			} else if hold.SellDate.Equal(bind.Shanghai.Date) {
				inv.TodayProfit = inv.TodayProfit.Add(hold.SellPrice.Sub(hold.BuyPrice).Mul(decimal.NewFromInt(100)))
				inv.TodayInvestCosts = inv.TodayInvestCosts.Add(inv.StockDaily.PreClose.Mul(decimal.NewFromInt(100)))
				inv.CumProfit = inv.CumProfit.Add(hold.SellPrice.Sub(hold.BuyPrice).Mul(decimal.NewFromInt(100)))
			} else { //之前卖出的
				inv.CumProfit = inv.CumProfit.Add(hold.SellPrice.Sub(hold.BuyPrice).Mul(decimal.NewFromInt(100)))
			}
		}
		if inv.TodayInvestCosts.GreaterThan(decimal.NewFromInt(0)){
			inv.TodayProfitRate = inv.TodayProfit.Div(inv.TodayInvestCosts).Mul(decimal.NewFromInt(100))
		}
		if inv.InvestCosts.GreaterThan(decimal.NewFromInt(0)){
			inv.CumProfitRate = inv.CumProfit.Div(inv.InvestCosts).Mul(decimal.NewFromInt(100))
		}
		bind.CumProfit = bind.CumProfit.Add(inv.CumProfit)
		bind.TodayProfit = bind.TodayProfit.Add(inv.TodayProfit)
		bind.TodayInvestCosts = bind.TodayInvestCosts.Add(inv.TodayInvestCosts)
		bind.InvestCosts = bind.InvestCosts.Add(inv.InvestCosts)
		bind.HoldAssets = bind.HoldAssets.Add(inv.HoldAssets)
		inv_list = append(inv_list, inv)
	}
	bind.HoldPositionList = inv_list

	//计算当日清仓
	inv_list = []Position{}
	for _, inv := range bind.TodayClosePostionList {
		//挪入历史持仓
		if inv.LastOperation.action == Close && inv.LastOperation.date.Before(bind.Shanghai.Date) {
			bind.ClosedPostionList = append(bind.ClosedPostionList, inv)
			continue
		}
		inv.TodayProfit = decimal.NewFromInt(0)
		inv.TodayInvestCosts = decimal.NewFromInt(0)
		inv.InvestCosts = decimal.NewFromInt(0)
		inv.CumProfit = decimal.NewFromInt(0)
		for _, hold := range inv.HoldLotsInfoList {
			if hold.SellDate.Equal(bind.Shanghai.Date) {
				inv.TodayProfit = inv.TodayProfit.Add(hold.SellPrice.Sub(inv.StockDaily.PreClose).Mul(decimal.NewFromInt(100)))
				inv.TodayInvestCosts = inv.TodayInvestCosts.Add(inv.StockDaily.PreClose.Mul(decimal.NewFromInt(100)))
			}
			inv.CumProfit = inv.CumProfit.Add(hold.SellPrice.Sub(hold.BuyPrice).Mul(decimal.NewFromInt(100)))
			inv.InvestCosts = inv.InvestCosts.Add(hold.BuyPrice.Mul(decimal.NewFromInt(100)))
		}
		inv.TodayProfitRate = inv.TodayProfit.Div(inv.TodayInvestCosts).Mul(decimal.NewFromInt(100))
		inv.CumProfitRate = inv.CumProfit.Div(inv.InvestCosts).Mul(decimal.NewFromInt(100))
		bind.CumProfit = bind.CumProfit.Add(inv.CumProfit)
		bind.TodayProfit = bind.TodayProfit.Add(inv.TodayProfit)
		bind.TodayInvestCosts = bind.TodayInvestCosts.Add(inv.TodayInvestCosts)
		inv_list = append(inv_list, inv)
	}
	bind.TodayClosePostionList = inv_list

	//计算历史清仓
	inv_list = []Position{}
	for _, inv := range bind.ClosedPostionList {
		inv.InvestCosts = decimal.NewFromInt(0)
		inv.CumProfit = decimal.NewFromInt(0)
		for _, hold := range inv.HoldLotsInfoList {
			inv.CumProfit = inv.CumProfit.Add(hold.SellPrice.Sub(hold.BuyPrice).Mul(decimal.NewFromInt(100)))
			inv.InvestCosts = inv.InvestCosts.Add(hold.BuyPrice.Mul(decimal.NewFromInt(100)))
		}
		inv.CumProfitRate = inv.CumProfit.Div(inv.InvestCosts).Mul(decimal.NewFromInt(100))
		bind.CumProfit = bind.CumProfit.Add(inv.CumProfit)
		inv_list = append(inv_list, inv)
	}
	bind.ClosedPostionList = inv_list

	//更新投资峰值记录
	if bind.InvestCosts.GreaterThan(bind.HighInvestCosts) {
		bind.HighInvestCosts = bind.InvestCosts
	}
	if !bind.HighInvestCosts.IsZero(){
		bind.HighInvestCostsRate = bind.CumProfit.Div(bind.HighInvestCosts).Mul(decimal.NewFromInt(100))
	}
	if !bind.TodayInvestCosts.IsZero() {
		bind.TodayProfitRate = bind.TodayProfit.Div(bind.TodayInvestCosts).Mul(decimal.NewFromInt(100))
	}
	bind.TotalAssets = bind.HoldAssets.Add(bind.LiquidAssets)
}

func (bind *Trading) GetHoldPositionList() []Position {
	return bind.HoldPositionList[:]
}

func (bind *Trading) Run() {
	// Excel报表
	//xlsx_filename := bind.StartDate + time.Now().Format(" 15:04:05") + ".xlsx"
	xlsx_filename := bind.StartDate + "-" + reflect.TypeOf(bind.strategy).Elem().Name() + ".xlsx"
	os.Remove(xlsx_filename)
	var xlsx *excelize.File
	if _, err := os.Lstat(xlsx_filename); os.IsNotExist(err) {
		if err := excelize.NewFile().SaveAs(xlsx_filename); err != nil {
			log.Error(err)
		}
	}
	xlsx, err := excelize.OpenFile(xlsx_filename)
	if err != nil {
		log.Error(err)
	}
	sh_index_daily_list := []IndexDaily{}
	GetDB().Debug().Where("code = ? and date between ? and ?", "000001", bind.StartDate, bind.EndDate).Order("date asc").Find(&sh_index_daily_list)
	keep_limit_up_index_daily_list := []IndexDaily{}
	GetDB().Debug().Where("code = ? and date between ? and ? and volume_ratio > 2", "90.BK0816", bind.StartDate, bind.EndDate).Order("date asc").Find(&keep_limit_up_index_daily_list)

	for sh_i, sh := range sh_index_daily_list {
		bind.ThinkDays = bind.ThinkDays-1
		bind.ThinkRate = bind.ThinkRate.Add(bind.TodayProfitRate)
		if bind.ThinkRate.GreaterThan(decimal.NewFromFloat(20)){
			bind.ThinkRate = decimal.NewFromFloat(0)
			bind.TodayProfitRate = decimal.NewFromFloat(0)
			bind.ThinkDays = 40
		}
		if bind.ThinkRate.LessThan(decimal.NewFromFloat(-2)){
			bind.ThinkRate = decimal.NewFromFloat(0)
			bind.TodayProfitRate = decimal.NewFromFloat(0)
			bind.ThinkDays = 5
		}

		bind.Shanghai = sh
		bind.IsIndexLargeVolume = false
		for _, ind := range keep_limit_up_index_daily_list {
			//if sh.Date.Equal(ind.Date){
			//	bind.IsIndexLargeVolume = true
			//	break
			//}
			d := sh.Date.Sub(ind.Date).Hours() / 24
			if d == 1 {
				bind.IsIndexLargeVolume = true
				break
			}
		}
		bind.IsRiskyDay = false
		if bind.isDayInRiskyDays(bind.Shanghai.Date) {
			bind.IsRiskyDay = true
		}

		bind.ComputeInvestingStock()

		//加减仓决策
		bind.strategy.DecisionPosition()
		//选股建仓
		bind.strategy.OpenPosition()

		bind.ComputeInvestingStock()

		sheet := bind.Shanghai.Date.Format("2006-01-02")
		xlsx.DeleteSheet(sheet)
		xlsx.NewSheet(sheet)
		fill_xlsx_cell(xlsx, sheet, 1, []interface{}{"代码", "名称", "开盘价", "收盘价", "涨幅", "建仓日期", "当日操作", "成本价", "持股", "可用", "净投入", "当日收益", "当日收益率", "累计收益", "累计收益率"})
		xlsx.SetColWidth(sheet, "A", "A", 12)
		xlsx.SetColWidth(sheet, "B", "B", 12)
		xlsx.SetColWidth(sheet, "F", "F", 12)
		xlsx.SetColWidth(sheet, "G", "G", 16)
		xlsx.SetColWidth(sheet, "H", "H", 12)
		xlsx.SetColWidth(sheet, "I", "I", 12)

		a := append(bind.HoldPositionList, bind.TodayClosePostionList...)
		position_list := []Position{}
		copier.Copy(&position_list, &a)

		sort.Sort(PositionList(position_list))

		for i, inv := range position_list {
			var action string
			if inv.LastOperation.date.Equal(bind.Shanghai.Date) {
				action = inv.LastOperation.action.String() + " " + inv.LastOperation.note
			}
			// 填充报表字段
			fill_xlsx_cell(xlsx, sheet, i+2, []interface{}{
				inv.StockDaily.Code,
				inv.StockDaily.Name,
				inv.StockDaily.Open,
				inv.StockDaily.Close,
				inv.StockDaily.PctChg.String() + "%",
				inv.OpenDate.Format("2006-01-02"),
				action,
				inv.InvestCosts.Div(decimal.NewFromInt(int64(len(inv.HoldLotsInfoList) * 100))).Round(2),
				len(bind.GetHold(inv)) * 100,
				len(bind.GetAvailableHold(inv)) * 100,
				inv.TodayInvestCosts.Round(2),
				inv.TodayProfit.Round(2),
				inv.TodayProfitRate.Round(2).String() + "%",
				inv.CumProfit.Round(2),
				inv.CumProfitRate.Round(2).String() + "%",
			})
		}
		//底部汇总
		col := strconv.Itoa(4 + len(position_list))
		xlsx.SetCellValue(sheet, "A"+col, "汇总")
		xlsx.SetCellValue(sheet, "K"+col, bind.InvestCosts.Round(2))
		xlsx.SetCellValue(sheet, "L"+col, bind.TodayProfit.Round(2))
		xlsx.SetCellValue(sheet, "M"+col, bind.TodayProfitRate.Round(2).String()+"%")

		//首页总览
		xlsx.SetColWidth("Sheet1", "A", "A", 12)
		xlsx.SetColWidth("Sheet1", "H", "H", 12)
		xlsx.SetCellValue("Sheet1", "A1", "日期")
		xlsx.SetCellValue("Sheet1", "B1", "净投入")
		xlsx.SetCellValue("Sheet1", "C1", "当日收益")
		xlsx.SetCellValue("Sheet1", "D1", "当日收益率")
		xlsx.SetCellValue("Sheet1", "E1", "累计收益")
		xlsx.SetCellValue("Sheet1", "F1", "初始资产收益率")
		xlsx.SetCellValue("Sheet1", "G1", "最高净投入")
		xlsx.SetCellValue("Sheet1", "H1", "最高净投入收益率")
		xlsx.SetCellValue("Sheet1", "A"+strconv.Itoa(sh_i+2), bind.Shanghai.Date.Format("2006-01-02"))
		xlsx.SetCellValue("Sheet1", "B"+strconv.Itoa(sh_i+2), bind.InvestCosts.Round(2))
		xlsx.SetCellValue("Sheet1", "C"+strconv.Itoa(sh_i+2), bind.TodayProfit.Round(2))
		xlsx.SetCellValue("Sheet1", "D"+strconv.Itoa(sh_i+2), bind.TodayProfitRate.Round(2).String()+"%")
		xlsx.SetCellValue("Sheet1", "E"+strconv.Itoa(sh_i+2), bind.CumProfit.Round(2))
		xlsx.SetCellValue("Sheet1", "F"+strconv.Itoa(sh_i+2), bind.TotalAssets.Sub(bind.OriginalAssets).Div(bind.OriginalAssets).Mul(decimal.NewFromInt(100)).Round(2).String()+"%")
		xlsx.SetCellValue("Sheet1", "G"+strconv.Itoa(sh_i+2), bind.HighInvestCosts.Round(2))
		xlsx.SetCellValue("Sheet1", "H"+strconv.Itoa(sh_i+2), bind.HighInvestCostsRate.Round(2).String()+"%")

		if len(position_list) > 0 {
			if err := xlsx.Save(); err != nil {
				log.Error(err)
			}
		} else {
			xlsx.DeleteSheet(sheet)
		}
		pr(bind.Shanghai.Date.Format("2006-01-02"), bind.TotalAssets, bind.TodayInvestCosts.Round(2), bind.TodayProfit.Round(2), bind.TodayProfitRate.Round(2), bind.TotalAssets.Sub(bind.OriginalAssets).Div(bind.OriginalAssets).Mul(decimal.NewFromInt(100)).Round(2).String()+"%")
	}
	log.Infof("%v 初始资金 %v, 期末资金 %v, 总收益率 %v%%, 最高净投入 %v", bind.StartDate, bind.OriginalAssets, bind.TotalAssets, bind.TotalAssets.Sub(bind.OriginalAssets).Div(bind.OriginalAssets).Mul(decimal.NewFromInt(100)).Round(2), bind.HighInvestCosts.Round(2))
}
