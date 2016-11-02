package monitor

import (
	"github.com/kyf/util/log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/kyf/martini"
)

const (
	PONG_WAIT   = 30 * time.Second
	PING_PERIOD = PONG_WAIT * 9 / 10
)

type Monitor struct {
	ch        <-chan string
	consumers []*Consumer
	sync.Mutex
}

func NewMonitor(ch <-chan string) *Monitor {
	return &Monitor{ch: ch, consumers: make([]*Consumer, 0)}
}

type Consumer struct {
	ch chan string
}

func NewConsumer(ch chan string) *Consumer {
	return &Consumer{ch}
}

func (this *Monitor) Register(con *Consumer) {
	this.Lock()
	defer this.Unlock()

	this.consumers = append(this.consumers, con)
}

func (this *Monitor) UnRegister(con *Consumer) {
	this.Lock()
	defer this.Unlock()

	log.Print("before remove size is ", len(this.consumers))
	tmp := make([]*Consumer, 0)
	for _, it := range this.consumers {
		if con != it {
			tmp = append(tmp, it)
		}
	}

	this.consumers = tmp
	log.Print("after remove size is ", len(this.consumers))
}

type MonitorServer struct {
	Routes   map[string]*Monitor
	WsServer *websocket.Upgrader
}

func NewMonitorServer(routes map[string]*Monitor) *MonitorServer {
	return &MonitorServer{
		Routes:   routes,
		WsServer: &websocket.Upgrader{ReadBufferSize: 1024, WriteBufferSize: 1024, CheckOrigin: func(r *http.Request) bool { return true }},
	}
}

func (this *MonitorServer) Run(logger *log.Logger) martini.Handler {
	for _, mon := range this.Routes {
		go func(m *Monitor) {
			for {
				select {
				case it := <-m.ch:
					m.Lock()
					for _, con := range m.consumers {
						con.ch <- it
					}
					m.Unlock()
				}
			}
		}(mon)
	}
	return func(w http.ResponseWriter, r *http.Request, context martini.Context) (result bool) {
		result = true
		for rawpath, mon := range this.Routes {
			if r.URL.Path == rawpath {
				conn, err := this.WsServer.Upgrade(w, r, nil)
				if err != nil {
					logger.Printf("WsServer Upgrade err:%v", err)
					conn.WriteMessage(websocket.CloseMessage, []byte{})
					return
				}
				defer conn.Close()
				ch := make(chan string, 1)
				self := NewConsumer(ch)
				mon.Register(self)
				defer mon.UnRegister(self)

				conn.SetPongHandler(func(string) error { conn.SetReadDeadline(time.Now().Add(PONG_WAIT)); return nil })

				for {
					select {
					case data := <-ch:
						err = conn.WriteMessage(websocket.TextMessage, []byte(data))
						if err != nil {
							logger.Printf("write message err:%v", err)
							return
						}
					case <-time.After(PING_PERIOD):
						if err := conn.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
							return
						}
					}
				}

				return
			}
		}
		result = false
		return
	}
}
