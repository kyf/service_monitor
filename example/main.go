package main

import (
	"log"
	"net/http"

	"github.com/kyf/martini"
	monitor "github.com/kyf/service_monitor"
	mlog "github.com/kyf/util/log"
)

const (
	LOG_DIR    = "/var/log/demo_monitor/"
	LOG_PREFIX = "[demo_monitor]"
)

func main() {
	logger, err := mlog.NewLogger(LOG_DIR, LOG_PREFIX, log.LstdFlags)
	if err != nil {
		log.Fatal(err)
	}
	chan_data1 := make(chan string, 1)
	monitor1 := monitor.NewMonitor(chan_data1)

	chan_data2 := make(chan string, 1)
	monitor2 := monitor.NewMonitor(chan_data2)

	routes := map[string]*monitor.Monitor{
		"/debug/monitor/data1": monitor1,
		"/debug/monitor/data2": monitor2,
	}
	monitorServer := monitor.NewMonitorServer(routes)
	m := martini.Classic()
	m.Map(logger)

	m.Get("/data1", func(r *http.Request) {
		chan_data1 <- r.RequestURI
	})

	m.Get("/data2", func(r *http.Request) {
		chan_data1 <- r.URL.Path
	})

	logger.SetAction(func(it string) {
		chan_data2 <- it
	})

	m.Use(monitorServer.Run(logger))
	m.RunOnAddr(":1212")
}
