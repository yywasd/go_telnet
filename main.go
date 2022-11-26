package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

const (
	TIME_DELAY_AFTER_WRITE = 500 //500ms
)

type TelnetClient struct {
	Ip       string
	Port     string
	IsAuth   bool
	Username string
	Password string
	Rc       chan string
	Wc       chan string
	Sc       chan os.Signal
}

func (tc *TelnetClient) GetAddr() string {
	return tc.Ip + ":" + tc.Port
}

func (tc *TelnetClient) IsOpen(timeout int) bool {
	addr := tc.GetAddr()
	conn, err := net.DialTimeout("tcp", addr, time.Duration(timeout)*time.Second)
	if err != nil {
		log.Println("Open errorInfo:", err)
		return false
	}
	defer conn.Close()
	return true
}

func (tc *TelnetClient) TelnetProtocolHandshake(conn net.Conn) bool {
	var buf [4096]byte

	// 第一次读
	n, err := conn.Read(buf[0:])
	if err != nil {
		log.Println("Handshake errorInfo:", err)
		return false
	}
	// 第一次写

	buf[1] = 252
	buf[4] = 252
	buf[7] = 252
	buf[10] = 252
	_, err = conn.Write(buf[0:n])
	if err != nil {
		log.Println("Handshake errorInfo:", err)
		return false
	}

	// 第二次读
	n, err = conn.Read(buf[0:])
	if err != nil {
		log.Println("Handshake errorInfo:", err)
		return false
	}
	// 第二次写
	buf[1] = 252
	buf[4] = 251
	buf[7] = 252
	buf[10] = 254
	buf[13] = 252
	_, err = conn.Write(buf[0:n])
	if err != nil {
		log.Println("Handshake errorInfo:", err)
		return false
	}

	// 第三次读
	n, err = conn.Read(buf[0:])
	if err != nil {
		log.Println("Handshake errorInfo:", err)
		return false
	}

	//第三次写
	buf[1] = 252
	buf[4] = 252
	_, err = conn.Write(buf[0:n])
	if err != nil {
		log.Println("Handshake errorInfo:", err)
		return false
	}

	// 第四次读
	_, err = conn.Read(buf[0:])
	if err != nil {
		log.Println("Handshake errorInfo:", err)
		return false
	}

	if !tc.IsAuth {
		return true
	}

	// 写入用户名
	_, err = conn.Write([]byte(tc.Username + "\n"))
	if err != nil {
		log.Println("Handshake errorInfo:", err)
		return false
	}

	// 等待固定时长
	time.Sleep(time.Millisecond * TIME_DELAY_AFTER_WRITE)

	_, err = conn.Read(buf[0:])
	if err != nil {
		log.Println("Handshake errorInfo:", err)
		return false
	}

	// 写入密码
	_, err = conn.Write([]byte(tc.Password + "\n"))
	if err != nil {
		log.Println("Handshake errorInfo:", err)
		return false
	}

	// 等待固定时长
	time.Sleep(time.Millisecond * TIME_DELAY_AFTER_WRITE)

	_, err = conn.Read(buf[0:])
	if err != nil {
		log.Println("Handshake errorInfo:", err)
		return false
	}

	return true
}

func (tc *TelnetClient) Telnet(timeout int) error {
	addr := tc.GetAddr()
	conn, err := net.DialTimeout("tcp", addr, time.Duration(timeout)*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()

	if !tc.TelnetProtocolHandshake(conn) {
		return errors.New("error")
	}

	go func() {
		for {
			data := make([]byte, 1024)
			n, err := conn.Read(data)
			if err != nil {
				fmt.Println(err)
				break
			}
			fmt.Printf("%s", data[0:n])
			tc.Rc <- string(data[0:n])
		}
	}()

	for {
		select {
		case cmd := <-tc.Wc:
			_, err = conn.Write([]byte(cmd + "\n"))
			if err != nil {
				log.Panicln(err)
				return err
			}

		case <-tc.Sc:
			conn.Close()
			return errors.New("对话正常结束。")

		default:
			time.Sleep(100 * time.Millisecond)
			continue
		}
	}

}

func NewTelnetClient(ip, port, username, password string, isauth bool) TelnetClient {
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, os.Interrupt, syscall.SIGTERM)
	return TelnetClient{
		Ip:       ip,
		Port:     port,
		IsAuth:   isauth,
		Username: username,
		Password: password,
		Rc:       make(chan string),
		Wc:       make(chan string),
		Sc:       sc,
	}
}

func main() {
	tc := NewTelnetClient("", "", "", "", true)

	go func() {
		err := tc.Telnet(20)
		if err != nil {
			fmt.Println(err)
		}
	}()

	go func() {
		for {
			message := <-tc.Rc
			fmt.Println(message)
		}
	}()

	for {
		line, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil && err != io.EOF {
			log.Fatal(err)
		}
		tc.Wc <- strings.TrimSpace(line)
	}
}
