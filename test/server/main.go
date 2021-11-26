package main

import (
	"bufio"
	"fmt"
	"net"
	"strings"
)

func process(conn net.Conn) {
	defer conn.Close()
	for {
		message, err := bufio.NewReader(conn).ReadString('\n')
		if err != nil {
			fmt.Println("read error : ", err)
			return
		}

		fmt.Print("Message Received:", string(message))
		newMessage := strings.ToUpper(message)
		conn.Write([]byte(newMessage + "\n"))
	}
}

func main() {
	// 监听TCP 服务端口
	listener, err := net.Listen("tcp", ":80")
	if err != nil {
		fmt.Println("Listen tcp server failed,err:", err)
		return
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Listen.Accept failed,err:", err)
			continue
		}

		go process(conn)
	}
}
