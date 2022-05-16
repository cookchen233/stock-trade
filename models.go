package main

import (
	"fmt"
	"github.com/shopspring/decimal"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
	"sync"
	"time"
)

var connections = make(map[string]*gorm.DB)
var con_mu sync.Mutex

func GetDB() *gorm.DB {
	dbname := "ch_stocks"
	_, ok := connections[dbname]
	if ok {
		return connections[dbname]
	}
	dsn := fmt.Sprintf("%s:%s@(%s)/%s?charset=utf8mb4&parseTime=True&loc=Local", "yezishui", "yezishui198312", "127.0.0.1", dbname)
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			TablePrefix:   "",
			SingularTable: true,
		},
		Logger: logger.Default.LogMode(logger.Error),
	})
	if err != nil {
		panic(err)
	}
	sqlDB, err := db.DB()

	// SetMaxIdleConns 设置空闲连接池中连接的最大数量
	sqlDB.SetMaxIdleConns(10)

	// SetMaxOpenConns 设置打开数据库连接的最大数量。
	sqlDB.SetMaxOpenConns(100)

	// SetConnMaxLifetime 设置了连接可复用的最大时间。
	sqlDB.SetConnMaxLifetime(time.Hour)
	connections[dbname] = db
	return db
}

type ModelData interface {
	GetTodayStockDaily(StockDaily StockDaily, date time.Time) StockDaily
}

type Daily struct {
	Id          int
	Code        string
	Name        string
	Date        time.Time
	Open        decimal.Decimal
	High        decimal.Decimal
	Low         decimal.Decimal
	Close       decimal.Decimal
	PctChg      decimal.Decimal
	Amplitude   decimal.Decimal
	Volume      int64
	VolumeRatio decimal.Decimal
	Amount      decimal.Decimal
	Turnover    decimal.Decimal
	FreeShares  int64
	MarketValue decimal.Decimal
	PreClose    decimal.Decimal
	PrePctChg   decimal.Decimal
	PreTurnover decimal.Decimal
	Avp5        decimal.Decimal
	Avp10        decimal.Decimal
	Avp20       decimal.Decimal
	Avp30       decimal.Decimal
	Avp60       decimal.Decimal
	Avp120      decimal.Decimal
	Avp5Chg   decimal.Decimal
	Avp20Chg5   decimal.Decimal
	Avp60Chg5   decimal.Decimal
	FieldM      int64
	FieldN      int64
	FieldI      decimal.Decimal
	FieldJ      decimal.Decimal
	FieldX      string
	FieldY      string

	Type            int64
}

type StockDaily struct {
	Daily

	Industry        string
	IsLimitUp       int64
	KeepLimitUpDays int64
}

type IndexDaily struct {
	Daily

	Follow     int64
	TopMembers string
	Members    string
}

type EtfDaily struct {
	Daily

	Follow     int64
	TopMembers string
	Members    string
}
