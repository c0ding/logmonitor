//https://github.com/itsmikej/imooc_logprocess
package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	TypeHandleLine = 0
	TypeErrNum     = 1
)

var TypeMonitorChan = make(chan int, 200)

type Message struct {
	TimeLocal                    time.Time
	BytesSent                    int
	Path, Method, Scheme, Status string
	UpstreamTime, RequestTime    float64
}

type LogProcess struct {
	read  Reader
	write Writer

	rc chan string
	wc chan *Message
}

type Reader interface {
	Read(rc chan string)
}

type Writer interface {
	Write(wc chan *Message)
}

type ReadFile struct {
	path string
}

func (r *ReadFile) Read(rc chan string) {
	// 实时读取模块

	var (
		file   *os.File
		err    error
		reader *bufio.Reader
		line   []byte
	)
	// 1，打开文件
	if file, err = os.Open(r.path); err != nil {
		panic(fmt.Sprintf("打开文件错误 : %s", err.Error()))
	}

	// 2，从文件末尾开始逐行读取
	file.Seek(0, 2) //将读取位置定到末尾
	reader = bufio.NewReader(file)
	for {
		if line, err = reader.ReadBytes('\n'); err != nil {
			if err == io.EOF {
				time.Sleep(500 * time.Millisecond)
				continue
			} else {
				panic(fmt.Sprintf("文件读取错误 : %s", err.Error()))
			}

		}

		// 3，写入 通道，供解析读取
		line = line[:len(line)-1] // 去掉最后一个字符：换行符
		rc <- string(line)
	}

}

type WriteFile struct {
	fileInfo string
}

func (w *WriteFile) Write(wc chan *Message) {
	//写入模块

	for v := range wc {
		fmt.Println(v)
	}
}

func (l *LogProcess) Process() {

	//解析模块

	/**
	'$remote_addr\t$http_x_forwarded_for\t$remote_user\t[$time_local]\t$scheme\t"$request"\t$status\t$body_bytes_sent\t"$http_referer"\t"$http_user_agent"\t"$gzip_ratio"\t$upstream_response_time\t$request_time'
	*/

	rep := regexp.MustCompile(`([\d\.]+)\s+([^ \[]+)\s+([^ \[]+)\s+\[([^\]]+)\]\s+([a-z]+)\s+\"([^"]+)\"\s+(\d{3})\s+(\d+)\s+\"([^"]+)\"\s+\"(.*?)\"\s+\"([\d\.-]+)\"\s+([\d\.-]+)\s+([\d\.-]+)`)

	//loc, _ := time.LoadLocation("Asia/Shanghai")

	//1，从读取通道中 读取每行日志数据
	//2，用正则表达，提取需要的数据，比如：path，status，Method
	//3，将数据写入 写入通道

	for v := range l.rc {
		ret := rep.FindStringSubmatch(v)
		if len(ret) != 14 {
			log.Fatal("正则失败", v)
			continue
		}

		message := &Message{}
		loc, _ := time.LoadLocation("Asia/Shanghai")
		t, err := time.ParseInLocation("02/Jan/2006:15:04:05 +0000", ret[4], loc)
		if err != nil {
			TypeMonitorChan <- TypeErrNum
			log.Println("ParseInLocation fail:", err.Error(), ret[4])
			continue
		}
		message.TimeLocal = t

		byteSent, _ := strconv.Atoi(ret[8])
		message.BytesSent = byteSent

		// GET /foo?query=t HTTP/1.0
		reqSli := strings.Split(ret[6], " ")
		if len(reqSli) != 3 {
			TypeMonitorChan <- TypeErrNum
			log.Println("strings.Split fail", ret[6])
			continue
		}
		message.Method = reqSli[0]

		u, err := url.Parse(reqSli[1])
		if err != nil {
			log.Println("url parse fail:", err)
			TypeMonitorChan <- TypeErrNum
			continue
		}
		message.Path = u.Path

		message.Scheme = ret[5]
		message.Status = ret[7]

		upstreamTime, _ := strconv.ParseFloat(ret[12], 64)
		requestTime, _ := strconv.ParseFloat(ret[13], 64)
		message.UpstreamTime = upstreamTime
		message.RequestTime = requestTime

		l.wc <- message
	}

}

func main() {
	var (
		LogPro *LogProcess
		w      *WriteFile
		r      *ReadFile
	)

	w = &WriteFile{
		fileInfo: "",
	}

	r = &ReadFile{
		path: "./access.log",
	}

	LogPro = &LogProcess{
		write: w,
		read:  r,
		wc:    make(chan *Message),
		rc:    make(chan string),
	}

	go LogPro.read.Read(LogPro.rc)
	go LogPro.Process()
	go LogPro.write.Write(LogPro.wc)

	time.Sleep(20 * time.Second)

}
