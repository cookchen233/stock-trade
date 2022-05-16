package main

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"github.com/pkg/errors"
	"github.com/rifflock/lfshook"
	"github.com/shopspring/decimal"
	log "github.com/sirupsen/logrus"
	"github.com/xuri/excelize"
	"gopkg.in/gomail.v2"
	"math"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

func ListDir_name(path string) []string {
	var files []string
	filepath.Walk(path, func(file string, info os.FileInfo, err error) error {
		files = append(files, file)
		return nil
	})
	return files

}

func pr(args ...interface{}) {
	fmt.Println(args...)
}

func Grange(min int, max int, step int) []int {
	var a []int
	if step < 0 {
		for i := min; i > max; i += step {
			a = append(a, i)
		}
	}
	for i := min; i < max; i += step {
		a = append(a, i)
	}
	return a
}

func Min(x, y int) int {
	if x < y {
		return x
	}
	return y
}

func Max(x, y int) int {
	if x > y {
		return x
	}
	return y
}

func Sec(sec int) time.Duration {
	return time.Duration(10) * time.Second
}

func Gmd5(str string) string {
	h := md5.New()
	h.Write([]byte(str))
	return hex.EncodeToString(h.Sum(nil))
}

type Glimit struct {
	n int
	c chan struct{}
}

// initialization Glimit struct
func NewGlimit(n int) Glimit {
	return Glimit{
		n: n,
		c: make(chan struct{}, n),
	}
}

// Run f in a new goroutine but with limit.
func (g *Glimit) Run(fn interface{}, args ...interface{}) {
	g.c <- struct{}{}
	go func() {
		defer func() { <-g.c }()
		v := reflect.ValueOf(fn)
		rargs := make([]reflect.Value, len(args))
		for i, a := range args {
			rargs[i] = reflect.ValueOf(a)
		}
		v.Call(rargs)
	}()
}

func SafeDefer(params ...interface{}) {
	defer func() {
		if r := recover(); r != nil {
			msg := fmt.Sprintf("%+v", r)
			stack := string(debug.Stack())
			log.Error(fmt.Sprintf("recovery from panic:\n%s\n%s", msg, stack), true)
			return
		}
	}()

	r := recover()
	if r == nil {
		return
	}

	err := fmt.Errorf("%+v", r)
	stack := string(debug.Stack())
	log.Error(fmt.Sprintf("recovery from panic:\n%s\n%s", err.Error(), stack), true)

	if paramsLen := len(params); paramsLen > 0 {
		if reflect.TypeOf(params[0]).String()[0:4] != "func" {
			return
		}
		var args []reflect.Value
		if paramsLen > 1 {
			args = append(args, reflect.ValueOf(err))
			for _, v := range params[1:] {
				args = append(args, reflect.ValueOf(v))
			}
		}
		reflect.ValueOf(params[0]).Call(args)
	}
}

func SafeGo(params ...interface{}) {
	if len(params) == 0 {
		return
	}

	pg := &panicGroup{panics: make(chan string, 1), dones: make(chan struct{}, 1)}
	defer pg.closeChan()

	go func() {
		defer func() {
			if r := recover(); r != nil {
				pg.panics <- fmt.Sprintf("%+v\n%s", r, string(debug.Stack()))
				return
			}
			pg.dones <- struct{}{}
		}()
		var args []reflect.Value
		if len(params) > 1 {
			for _, v := range params[1:] {
				args = append(args, reflect.ValueOf(v))
			}
		}
		reflect.ValueOf(params[0]).Call(args)
	}()

	for {
		select {
		case <-pg.dones:
			return
		case p := <-pg.panics:
			panic(p)
		}
	}
}

// PanicGroup 安全的 Go
type PanicGroup interface {
	Go(...interface{}) *panicGroup
	Wait()
}

// @title 实例化 PanicGroup
// @param limit 并发协程数量限制
func NewSafeGo(limit int) PanicGroup {
	p := &panicGroup{
		panics: make(chan string, 1),
		dones:  make(chan struct{}, limit),
		limit:  make(chan struct{}, limit),
	}
	p.Go()
	return p
}

type panicGroup struct {
	panics chan string   // 协程 panic 通知通道
	dones  chan struct{} // 协程完成通知通道
	jobN   int32         // 协程计数
	limit  chan struct{} //限制器
}

func (g *panicGroup) Go(params ...interface{}) *panicGroup {
	if len(params) == 0 {
		params = []interface{}{func() {}}
	}
	atomic.AddInt32(&g.jobN, 1)
	go func() {
		defer func() {
			<-g.limit
			if r := recover(); r != nil {
				func() {
					defer func() {
						if r := recover(); r != nil {
						}
					}()
					g.panics <- fmt.Sprintf("%+v\n%s", r, string(debug.Stack()))
				}()
			}

			func() {
				defer func() {
					if r := recover(); r != nil {
					}
				}()
				g.dones <- struct{}{}
			}()
		}()
		g.limit <- struct{}{}
		var args []reflect.Value
		if len(params) > 1 {
			for _, v := range params[1:] {
				args = append(args, reflect.ValueOf(v))
			}
		}
		reflect.ValueOf(params[0]).Call(args)
	}()
	return g
}

func (g *panicGroup) Wait() {
	defer g.closeChan()
	for {
		select {
		case <-g.dones:
			if atomic.AddInt32(&g.jobN, -1) == 0 {
				return
			}
		case p := <-g.panics:
			panic(p)
		}
	}
}

func (g *panicGroup) closeChan() {
	close(g.dones)
	close(g.panics)
}

func IsFile(filename string) bool {
	file, err := os.Stat(filename)
	if err != nil {
		return false
	}
	return !file.IsDir()
}

func IsDir(filename string) bool {
	file, err := os.Stat(filename)
	if err != nil {
		return false
	}
	return file.IsDir()
}

func ByteCountBinary(size int64) string {
	const unit int64 = 1024
	if size < unit {
		return fmt.Sprintf("%dB", size)
	}
	div, exp := unit, 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%cB", float64(size)/float64(div), "KMGTPE"[exp])
}

func FileSize(filename string) int64 {
	file, err := os.Stat(filename)
	if err != nil {
		return 0
	}
	return file.Size()
}

func PartFilename(filename string) string {
	return path.Join(path.Base(path.Dir(filename)), path.Base(filename))
}

func RotateLogHook(logPath string, logFileName string, maxAge time.Duration, rotationTime time.Duration) *lfshook.LfsHook {
	baseLogPath := path.Join(logPath, logFileName)

	writer, err := rotatelogs.New(
		baseLogPath+".%Y-%m-%d",
		rotatelogs.WithLinkName(baseLogPath),      // 生成软链，指向最新日志文
		rotatelogs.WithMaxAge(maxAge),             // 文件最大保存时间
		rotatelogs.WithRotationTime(rotationTime), // 日志切割时间间隔
	)
	err_writer, err := rotatelogs.New(
		baseLogPath+".%Y-%m-%d",
		rotatelogs.WithLinkName(baseLogPath),      // 生成软链，指向最新日志文
		rotatelogs.WithMaxAge(maxAge),             // 文件最大保存时间
		rotatelogs.WithRotationTime(rotationTime), // 日志切割时间间隔
	)
	if err != nil {
		log.Errorf("config local file system logger error. %+v", errors.WithStack(err))
	}
	return lfshook.NewHook(lfshook.WriterMap{
		log.DebugLevel: writer, // 为不同级别设置不同的输出目的
		log.InfoLevel:  writer,
		log.WarnLevel:  writer,
		log.ErrorLevel: err_writer,
		log.FatalLevel: err_writer,
		log.PanicLevel: err_writer,
	}, &MineFormatter{})

}

type MineFormatter struct{}

const TimeFormat = "2006-01-02 15:04:05"

func (s *MineFormatter) Format(entry *log.Entry) ([]byte, error) {
	msg := fmt.Sprintf("[%s] [%s] %s\n", time.Now().Local().Format(TimeFormat), strings.ToUpper(entry.Level.String()), entry.Message)
	if entry.Level <= log.ErrorLevel {
		msg = fmt.Sprintf("[%s] [%s] %s\n%s\n", time.Now().Local().Format(TimeFormat), strings.ToUpper(entry.Level.String()), entry.Message, entry.Data["stack"])
	}

	return []byte(msg), nil
}

type MailHook struct {
	User      string
	Pass      string
	Host      string
	Port      string
	Receivers []string
}

func (hook *MailHook) Fire(entry *log.Entry) error {
	subject := "录音文件转文本数据发生错误"
	body := fmt.Sprintf("<h2>%s</h2><p>%s<p>", entry.Message, entry.Data["stack"])
	arr := strings.Split(body, "\n")
	body = strings.Join(arr, "</p><p>")
	return hook.send_mail(hook.Receivers, subject, body)
}

func (hook *MailHook) Levels() []log.Level {
	return []log.Level{
		log.PanicLevel,
		log.FatalLevel,
		log.ErrorLevel,
	}
}

func (hook *MailHook) SendMail(mailTo []string, subject string, body string) error {
	m := gomail.NewMessage()
	m.SetHeader("From", m.FormatAddress(hook.User, "Golang App Error")) //这种方式可以添加别名，即“XX官方”
	//说明：如果是用网易邮箱账号发送，以下方法别名可以是中文，如果是qq企业邮箱，以下方法用中文别名，会报错，需要用上面此方法转码
	//m.SetHeader("From", "FB Sample"+"<"+mailConn["user"]+">") //这种方式可以添加别名，即“FB Sample”， 也可以直接用m.SetHeader("From",mailConn["user"])
	//m.SetHeader("From", mailConn["user"])
	reg1 := regexp.MustCompile(`(.*?)<(.*?)>`)
	var to []string
	for _, v := range mailTo {
		res := reg1.FindAllStringSubmatch(v, -1)
		if len(res) > 0 {
			to = append(to, m.FormatAddress(res[0][2], res[0][1]))
		}
	}
	m.SetHeader("To", to...)
	m.SetHeader("Subject", subject)
	m.SetBody("text/html", body)
	port, _ := strconv.Atoi(hook.Port)
	d := gomail.NewDialer(hook.Host, port, hook.User, hook.Pass)
	err := d.DialAndSend(m)
	return err

}

// GetBetweenDates 根据开始日期和结束日期计算出时间段内所有日期
// 参数为日期格式，如：2020-01-01
func GetRangeDates(sdate, edate string) []interface{} {
	var d []interface{}
	timeFormatTpl := "2006-01-02 15:04:05"
	if len(timeFormatTpl) != len(sdate) {
		timeFormatTpl = timeFormatTpl[0:len(sdate)]
	}
	date, err := time.Parse(timeFormatTpl, sdate)
	if err != nil {
		// 时间解析，异常
		return d
	}
	date2, err := time.Parse(timeFormatTpl, edate)
	if err != nil {
		// 时间解析，异常
		return d
	}
	if date2.Before(date) {
		// 如果结束时间小于开始时间，异常
		return d
	}
	// 输出日期格式固定
	timeFormatTpl = "2006-01-02"
	date2Str := date2.Format(timeFormatTpl)
	d = append(d, date.Format(timeFormatTpl))
	for {
		date = date.AddDate(0, 0, 1)
		dateStr := date.Format(timeFormatTpl)
		d = append(d, dateStr)
		if dateStr == date2Str {
			break
		}
	}
	return d
}

func ArrayChunk(a []interface{}, c int) [][]interface{} {
	r := (len(a) + c - 1) / c
	b := make([][]interface{}, r)
	lo, hi := 0, c
	for i := range b {
		if hi > len(a) {
			hi = len(a)
		}
		b[i] = a[lo:hi:hi]
		lo, hi = hi, hi+c
	}
	return b
}

func InterfaceToString(inter interface{}) {

	switch inter.(type) {

	case string:
		fmt.Println("string", inter.(string))
		break
	case int:
		fmt.Println("int", inter.(int))
		break
	case float64:
		fmt.Println("float64", inter.(float64))
		break
	}

}

func Prf(format string, a ...interface{}) (n int, err error) {
	return fmt.Printf(format+"\n", a...)
}

func MinDecimal(x, y decimal.Decimal) decimal.Decimal {
	if x.LessThan(y) {
		return x
	}
	return y
}
func MaxDecimal(x, y decimal.Decimal) decimal.Decimal {
	if x.GreaterThan(y) {
		return x
	}
	return y
}

func InArray(e interface{}, s interface{}) bool {
	switch e.(type) {
	case int64:
		for _, a := range s.([]int64) {
			if a == e {
				return true
			}
		}
	case string:
		for _, a := range s.([]string) {
			if a == e {
				return true
			}
		}
	case float64:
		for _, a := range s.([]float64) {
			if a == e {
				return true
			}
		}
	case time.Time:
		for _, a := range s.([]time.Time) {
			if a.Equal(e.(time.Time)) {
				return true
			}
		}
	default:
		for _, a := range s.([]int64) {
			if a == e {
				return true
			}
		}
	}
	return false
}

func ArrayChunks(arr []string, num int64) [][]string {
	max := int64(len(arr))
	if max < num {
		return nil
	}
	var segmens = make([][]string, 0)
	quantity := max / num
	end := int64(0)
	for i := int64(1); i <= num; i++ {
		qu := i * quantity
		if i != num {
			segmens = append(segmens, arr[i-1+end:qu])
		} else {
			segmens = append(segmens, arr[i-1+end:])
		}
		end = qu - i
	}
	return segmens
}

func Gaussian(arr []decimal.Decimal) decimal.Decimal {
	//数学期望
	var sum decimal.Decimal
	for _, v := range arr {
		sum = sum.Add(v)
	}
	μ, _ := sum.Div(decimal.NewFromInt(int64(len(arr)))).Float64()

	//标准差
	var variance float64
	for _, v := range arr {
		v, _ := v.Float64()
		variance += math.Pow((v - μ), 2)
	}
	σ := math.Sqrt(variance / float64(len(arr)))
	fmt.Println("σ:", σ)
	fmt.Println("μ:", μ)

	//正态分布公式
	a := 1 / (math.Sqrt(2*math.Pi) * σ) * math.Pow(math.E, (-math.Pow((μ-μ), 2)/(2*math.Pow(σ, 2))))
	return decimal.NewFromFloat(a)
}

func sub_days(t1, t2 time.Time) (day int) {
	swap := false
	if t1.Unix() < t2.Unix() {
		t_ := t1
		t1 = t2
		t2 = t_
		swap = true
	}

	day = int(t1.Sub(t2).Hours() / 24)

	// 计算在被24整除外的时间是否存在跨自然日
	if int(t1.Sub(t2).Milliseconds())%86400000 > int(86400000-t2.Unix()%86400000) {
		day += 1
	}

	if swap {
		day = -day
	}

	return day
}

func exec_lock(locker sync.Mutex, fn interface{}, args ...interface{}) {
	locker.Lock()
	defer func() { locker.Unlock() }()
	v := reflect.ValueOf(fn)
	rargs := make([]reflect.Value, len(args))
	for i, a := range args {
		rargs[i] = reflect.ValueOf(a)
	}
	v.Call(rargs)
}

func fill_xlsx_cell(xlsx *excelize.File, sheet_name string, col int, value_list []interface{}) {
	abc := []string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L", "M", "N", "O", "P", "Q", "R", "S", "T", "U", "V", "W", "X", "Y", "Z"}
	for i, v := range value_list {
		xlsx.SetCellValue(sheet_name, abc[i]+strconv.Itoa(col), v)
	}
}
