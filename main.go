package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"math"
	"net"
	"os"
	"time"
)

var (
	timeout      int64                 // 超时时间
	size         int                   // 请求发送缓冲区大小
	count        int                   // 发送请求数
	typ          uint8 = 8             // ICMP报文类型，8为请求报文，0为应答报文
	code         uint8 = 0             // ICMP报文代码，0为请求报文，0为应答报文
	sendCount    int                   // 发送请求数
	successCount int                   // 成功请求数
	failCount    int                   // 失败请求数
	minTs        int64 = math.MaxInt32 // 最小延迟
	maxTs        int64 = 0             // 最大延迟
	totalTs      int64 = 0             // 总延迟
)

// 严格遵循ICMP报文结构顺序
type ICMP struct {
	Type     uint8  // 8位类型
	Code     uint8  // 8位代码
	CheckSum uint16 // 16位校验和
	ID       uint16 // 16位标识符
	Sequence uint16 // 16位序列号
}

func main() {
	getCommandArgs()
	destIp := os.Args[len(os.Args)-1]
	conn, err := net.DialTimeout("ip:icmp", destIp, time.Duration(timeout)*time.Millisecond)
	defer conn.Close()
	if err != nil {
		log.Fatal(err)
		return
	}
	fmt.Printf("正在 Ping %s [%s] 具有 %d 字节的数据:", destIp, conn.RemoteAddr(), size)
	for i := 0; i < count; i++ {
		sendCount++
		icmpPacket := &ICMP{
			Type:     typ,
			Code:     code,
			CheckSum: 0,
			ID:       1,
			Sequence: 1,
		}
		data := make([]byte, size)                          // 创建指定大小的字节切片
		var buffer bytes.Buffer                             // 创建字节缓冲区
		binary.Write(&buffer, binary.BigEndian, icmpPacket) // 将 ICMP 结构体的二进制表示写入缓冲区
		buffer.Write(data)                                  // 将随机数据附加到缓冲区
		data = buffer.Bytes()                               // 将缓冲区中的字节设置为 data 切片
		checkSum := checkSum(data)                          // 计算校验和
		data[2] = byte(checkSum >> 8)                       // 将校验和的高位字节写入 data 切片
		data[3] = byte(checkSum)                            // 将校验和的低位字节写入 data 切片

		conn.SetDeadline(time.Now().Add(time.Duration(timeout) * time.Millisecond))
		t1 := time.Now()
		n, err := conn.Write(data)
		if err != nil {
			failCount++
			log.Fatal(err)
			continue
		}
		buf := make([]byte, 65535)
		n, err = conn.Read(buf)
		if err != nil {
			failCount++
			log.Fatal(err)
			continue
		}
		successCount++
		ts := time.Since(t1).Milliseconds()
		if minTs > ts || minTs == math.MaxInt32 {
			minTs = ts
		}
		if maxTs < ts {
			maxTs = ts
		}
		totalTs += ts
		fmt.Printf("来自 %s 的回复: 字节=%d 时间=%dms TTL=%d\n", conn.RemoteAddr(), n-28, ts, buf[8])
		time.Sleep(time.Second)
	}
	fmt.Printf(`%s 的 Ping 统计信息:
    数据包: 已发送 = %d，已接收 = %d，丢失 = %d (%.2f%% 丢失)，
往返行程的估计时间(以毫秒为单位):
    最短 = %dms，最长 = %dms，平均 = %dms`,
		conn.RemoteAddr(),
		sendCount, successCount,
		failCount,
		float64(failCount)/float64(sendCount),
		minTs,
		maxTs,
		totalTs/int64(sendCount))
}

func checkSum(data []byte) uint16 {
	length := len(data)
	index := 0
	var sum uint32 = 0
	for length > 1 {
		sum += uint32(data[index])<<8 + uint32(data[index+1])
		index += 2
		length -= 2
	}
	if length != 0 {
		sum += uint32(data[index])
	}
	hi16 := sum >> 16
	for hi16 != 0 {
		sum = hi16 + uint32(uint16(sum))
		hi16 = sum >> 16
	}
	return uint16(^sum)
}

func getCommandArgs() {
	flag.Int64Var(&timeout, "w", 1000, "请求超时时长，单位毫秒")
	flag.IntVar(&size, "l", 32, "请求发送缓冲区大小，单位字节")
	flag.IntVar(&count, "n", 4, "发送请求数")
	flag.Parse()
}
