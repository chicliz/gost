package gost

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"github.com/go-log/log"
)

// Accepter represents a network endpoint that can accept connection from peer.
type Accepter interface {
	Accept() (net.Conn, error)
}

// Server is a proxy server.
type Server struct {
	Listener Listener
	Handler  Handler
	options  *ServerOptions
}

// Init intializes server with given options.
func (s *Server) Init(opts ...ServerOption) {
	if s.options == nil {
		s.options = &ServerOptions{}
	}
	for _, opt := range opts {
		opt(s.options)
	}
}

// Addr returns the address of the server
func (s *Server) Addr() net.Addr {
	return s.Listener.Addr()
}

// Close closes the server
func (s *Server) Close() error {
	return s.Listener.Close()
}

// Serve serves as a proxy server.
func (s *Server) Serve(h Handler, opts ...ServerOption) error {
	s.Init(opts...)

	if s.Listener == nil {
		ln, err := TCPListener("")
		if err != nil {
			return err
		}
		s.Listener = ln
	}

	if h == nil {
		h = s.Handler
	}
	if h == nil {
		h = HTTPHandler()
	}

	l := s.Listener
	var tempDelay time.Duration
	for {
		conn, e := l.Accept()
		if e != nil {
			if ne, ok := e.(net.Error); ok && ne.Temporary() {
				if tempDelay == 0 {
					tempDelay = 5 * time.Millisecond
				} else {
					tempDelay *= 2
				}
				if max := 1 * time.Second; tempDelay > max {
					tempDelay = max
				}
				log.Logf("server: Accept error: %v; retrying in %v", e, tempDelay)
				time.Sleep(tempDelay)
				continue
			}
			return e
		}
		tempDelay = 0

		go h.Handle(conn)
	}
}

// Run starts to serve.
func (s *Server) Run() error {
	return s.Serve(s.Handler)
}

// ServerOptions holds the options for Server.
type ServerOptions struct {
}

// ServerOption allows a common way to set server options.
type ServerOption func(opts *ServerOptions)

// Listener is a proxy server listener, just like a net.Listener.
type Listener interface {
	net.Listener
}

//rw1 监听端口, rw2 转发端口
//监听端口 (tcp) -> 转发端口 加数据
//监听端口 (tcp) <- 转发端口 减少数据
func transportTcp(rw1, rw2 io.ReadWriter) error {
	fmt.Println("listen tcp transport")
	errc := make(chan error, 1)
	go func() {
		//读取数据到监听端口
		errc <- copyBufferDel(rw1, rw2)
	}()

	go func() {
		//写入数据到h2客户端
		errc <- copyBufferAdd(rw2, rw1)
	}()

	err := <-errc
	if err != nil && err == io.EOF {
		err = nil
	}
	return err
}

//rw1 监听端口, rw2 转发端口
//监听端口 (http) -> 转发端口 减数据
//监听端口 (http) <- 转发端口 加数据
func transportHttp(rw1, rw2 io.ReadWriter) error {
	fmt.Println("listen http transport")
	errc := make(chan error, 1)
	go func() {
		//读取数据到监听端口
		errc <- copyBufferAdd(rw1, rw2)
	}()

	go func() {
		//写入数据到客户端
		errc <- copyBufferDel(rw2, rw1)
	}()

	err := <-errc
	if err != nil && err == io.EOF {
		err = nil
	}
	return err
}

func copyBufferAdd(dst io.Writer, src io.Reader) error {
	for {
		// will listen for message to process ending in newline (\n)
		message, err := bufio.NewReader(src).ReadString('\n')
		if err != nil {
			fmt.Println("read error : ", err)
			return err
		}

		message = "extern " + message + "\n"
		fmt.Print("add: Message Send:", message)

		_, err = dst.Write([]byte(message))
		if err != nil {
			fmt.Println("read error : ", err)
			return err
		}
	}
}

func copyBufferDel(dst io.Writer, src io.Reader) error {
	for {
		// will listen for message to process ending in newline (\n)
		message, err := bufio.NewReader(src).ReadString('\n')
		if err != nil {
			fmt.Println("read error : ", err)
			return err
		}

		if strings.HasPrefix(message, "extern ") {
			message = strings.TrimPrefix(string(message), "extern ")
		} else if strings.HasPrefix(message, "EXTERN ") {
			message = strings.TrimPrefix(string(message), "EXTERN ")
		}

		fmt.Print("del: Message Send:", message)

		_, err = dst.Write([]byte(message + "\n"))
		if err != nil {
			fmt.Println("read error : ", err)
			return err
		}
	}
}

func transport(rw1, rw2 io.ReadWriter) error {
	errc := make(chan error, 1)
	go func() {
		errc <- copyBuffer(rw1, rw2)
	}()

	go func() {
		errc <- copyBuffer(rw2, rw1)
	}()

	err := <-errc
	if err != nil && err == io.EOF {
		err = nil
	}
	return err
}

func copyBuffer(dst io.Writer, src io.Reader) error {
	buf := lPool.Get().([]byte)
	defer lPool.Put(buf)

	_, err := io.CopyBuffer(dst, src, buf)
	return err
}
