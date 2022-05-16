package main

import (
	"encoding/gob"
	logrus_stack "github.com/Gurpartap/logrus-stack"
	_ "github.com/jinzhu/gorm/dialects/mysql"
	"github.com/joho/godotenv"
	"github.com/patrickmn/go-cache"
	"github.com/shopspring/decimal"
	log "github.com/sirupsen/logrus"
	"os"
	"strings"
	"sync"
	"time"
)

var handled_num int
var ca *cache.Cache
var ne = cache.NoExpiration
var loc, _ = time.LoadLocation("Asia/Shanghai")

func init() {
	godotenv.Load()
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
	go func() {
		for {
			time.Sleep(time.Duration(60) * time.Second)
			File, _ := os.OpenFile(cache_file, os.O_RDWR|os.O_CREATE, 0777)
			defer File.Close()
			enc := gob.NewEncoder(File)
			if err := enc.Encode(ca.Items()); err != nil {
				log.Error(err)
			}
		}
	}()

	//log.SetLevel(log.DebugLevel)
	log.SetLevel(log.InfoLevel)

	callerLevels := []log.Level{
		log.PanicLevel,
		log.FatalLevel,
		log.ErrorLevel,
	}
	stackLevels := []log.Level{log.PanicLevel, log.FatalLevel, log.ErrorLevel}

	log.AddHook(logrus_stack.NewHook(callerLevels, stackLevels))

	log.AddHook(RotateLogHook("log", "stdout.log", 7*24*time.Hour, 24*time.Hour))

	if os.Getenv("environment") == "pro" {
		log.AddHook(&MailHook{
			os.Getenv("mail_user"),
			os.Getenv("mail_pass"),
			os.Getenv("mail_host"),
			os.Getenv("mail_port"),
			strings.Split(os.Getenv("err_receivers"), ";"),
		})
	}

}

type A struct {
	name string
	t    time.Time
}
type List []A

func (x List) Len() int           { return len(x) }
func (x List) Less(i, j int) bool { return x[i].t.Before(x[j].t) }
func (x List) Swap(i, j int)      { x[i], x[j] = x[j], x[i] }

var(
	global sync.WaitGroup
	wg sync.WaitGroup
	ch  = make(chan []int, 1)
)

func main() {
	//a_list := []A{
	//	A{
	//		name : "first",
	//		t : time.Now(),
	//	},
	//	A{
	//		name : "second",
	//		t : time.Now(),
	//	},
	//}
	//b_list := []A{
	//	A{
	//		name : "third",
	//		t : time.Now(),
	//	},
	//}
	//
	//c_list := append(a_list, b_list...)
	//sort.Sort(List(c_list))
	//
	//fmt.Println(len(c_list))

	//defer func() {
	//	if err := recover(); err != nil {
	//		log.Error("发生错误", err)
	//		time.Sleep(time.Second * 600)
	//		main()
	//	}
	//}()

	dates := [][]string{
		//{"2016-01-01", "2016-12-31",},
		//{"2017-01-01", "2017-12-31",},
		//{"2018-01-01", "2018-12-31",},
		//{"2019-01-01", "2019-12-31",},
		//{"2020-01-01", "2020-12-31",},
		{"2021-01-01", "2021-12-31"},
	}
	safe_go := NewSafeGo(50)
	for _, dt := range dates {
		safe_go.Go(func(dt []string) {
			trading := NewTrading(decimal.NewFromInt(200000), dt[0], dt[1])
			trading.SetStrategy(&StrategyB{&trading})
			//trading.SetStrategy(&Strategy2020{&trading})
			//trading.SetStrategy(&StrategyEtf{&trading})
			trading.Run()
		}, dt)
	}
	safe_go.Wait()

}
