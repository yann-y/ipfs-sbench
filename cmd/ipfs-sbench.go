package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"flag"
	"io"
	"io/ioutil"
	"ipfs-sbench/internal/conf"
	mr "math/rand"
	"sync"
	"time"

	shell "github.com/ipfs/go-ipfs-api"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

//var ipfs = shell.NewShell("192.168.1.141:5001")

// var tg = make(chan time.Duration, 10000)
var (
	configFileLocation string             // 上传文件的数目
	fileNum            int                // 文件数目
	tg                 chan time.Duration // 记录每一个文件的耗时
	ipfs               *shell.Shell       // ipfs链接地址
	size               int64              // 文件大小
	threadCount        int                // 线程数量
	cidCh              chan string        // 存储cid,方便下载
	rtime              chan time.Duration // 读取耗时
	rNumber            int                // 读取次数
	wNumber            int                // 写入次数
	cid                string             // 下载文件cid
)

func main() {
	flag.Parse()
	configFileContent, err := ioutil.ReadFile(configFileLocation)
	if err != nil {
		log.WithError(err).Fatalf("Error reading config file:")
	}
	config := loadConfigFromFile(configFileContent)
	// 检查配置文件是符合测试标准
	err = conf.CheckTestCase(&config)
	if err != nil {
		panic(err.Error())
	}
	ipfs = shell.NewShell(config.Tests.Ipfs)
	if ipfs == nil {
		panic("请检查ipfs配置,ipfs无法链接!!!")
	}
	fileNum = config.Tests.File.Number
	tg = make(chan time.Duration, fileNum)
	cidCh = make(chan string, fileNum)
	// 计算文件大小
	switch config.Tests.File.Unit {
	case "KB", "kb":
		size = int64(config.Tests.File.Size) * 1024
	case "MB", "mb":
		size = int64(config.Tests.File.Size) * 1024 * 1024
	case "GB", "G", "gb":
		size = int64(config.Tests.File.Size) * 1024 * 1024 * 1024
	default:
		size = int64(config.Tests.File.Size)
	}
	threadCount = config.Tests.ThreadCount

	//混合读写，考虑权重问题
	wNumber = fileNum

	rNumber = int((fileNum * config.Tests.File.ReadWeight) / config.Tests.File.WriteWeight)
	//fmt.Println(rNumber)
	rtime = make(chan time.Duration, rNumber)
	// 正式测试
	test4KFile(uint64(size))
}

// 加载配置文件，直接yaml序列化
func loadConfigFromFile(configFileContent []byte) conf.TestConf {
	var config conf.TestConf
	err := yaml.Unmarshal(configFileContent, &config)
	if err != nil {
		log.WithError(err).Fatalf("Error unmarshaling config file:")
	}
	return config
}

func init() {
	//读取配置文件
	flag.StringVar(&configFileLocation, "conf", "configs/config.yml", "config path, eg: -conf config.yaml")

}
func test4KFile(fileSize uint64) {
	queue := make(chan int8, 10)
	queue <- 0
	go func() {
		w, r := wNumber-1, rNumber
		mr.Seed(time.Now().UnixNano())
		for i := 0; i < wNumber+rNumber-1; i++ {
			n := mr.Intn(2)
			//fmt.Println(n)
			queue <- int8(n)
			if n == 0 {
				w--
			} else if n == 1 {
				r--
			}
			if w == 0 || r == 0 {
				break
			}

		}
		for i := 0; i < w; i++ {
			queue <- 0
		}
		for i := 0; i < r; i++ {
			queue <- 1
		}
	}()
	wg := sync.WaitGroup{}
	ch := make(chan struct{}, threadCount) // 限制并发的的数量
	for i := 0; i < wNumber+rNumber; i++ {
		wg.Add(1)
		ch <- struct{}{} // 通道满了，就阻塞
		go func() {
			defer wg.Done()
			switch <-queue {
			case 0:
				upLoad(fileSize)
			case 1:
				if len(cidCh) == 0 && cid != "" {
					getObject(cid)
				} else {
					cid = <-cidCh
					getObject(cid)
				}
			}
			<-ch
		}()
	}
	wg.Wait()
	close(tg) // 写完通道关闭，避免后续遍历阻塞。
	close(rtime)
	//fmt.Println(len(tg))
	// 计算成功了多少个文件
	wLen := len(tg)
	rLen := len(rtime)
	t1 := time.Now()
	t2 := t1
	//fmt.Println(t1, t2)
	for v := range tg {
		t1 = t1.Add(v)
	}
	takeUpTime := t1.Sub(t2)
	avg := takeUpTime / time.Duration(fileNum)
	rTimeCount := time.Duration(0)
	for v := range rtime {
		rTimeCount += v
	}
	rAvg := rTimeCount / time.Duration(rNumber)
	log.Infof("上传文件总数%d,文件大小%d", fileNum, size)
	log.Infof("写入文件总耗时%s,平均写入时间%s,成功率%0.2f", takeUpTime, avg.String(), float32(wLen/wNumber))
	log.Infof("读取文件总耗时%s,平均读取时间%s,成功率%0.2f", rTimeCount, rAvg.String(), float32(rLen/rNumber))
}

// 上传文件，记录耗时
func upLoad(fileSize uint64) {
	f := bytes.NewBuffer(generateRandomBytes(fileSize))
	start := time.Now()
	cid, err := ipfs.Add(f)
	if err != nil {
		log.WithError(err).Fatal("请求ipfs失败")
		return
	}
	duration := time.Since(start)
	log.Infof("文件大小：%d---->文件cid: %s--->文件耗时：%s", fileSize, cid, duration.String())
	tg <- duration
	if cid != "" {
		cidCh <- cid
	}
}

// 获取文件
func getObject(cid string) error {
	start := time.Now()
	resp, err := ipfs.Request("cat", cid).Send(context.Background())
	if err != nil {
		return err
	}
	if resp.Error != nil {
		return resp.Error
	}
	b := make([]byte, 1024)
	for {
		_, err := resp.Output.Read(b)
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Errorln(err)
			break
		}
	}
	duration := time.Since(start)
	rtime <- duration
	log.Infof("文件cid: %s--->文件耗时：%s", cid, duration.String())
	return nil
}

// 随机生成文件
func generateRandomBytes(size uint64) []byte {
	now := time.Now()
	random := make([]byte, size)
	n, err := rand.Read(random)
	if err != nil {
		log.WithError(err).Fatal("I had issues getting my random bytes initialized")
	}
	log.Debugf("Generated %d random bytes in %v", n, time.Since(now))
	return random
}
