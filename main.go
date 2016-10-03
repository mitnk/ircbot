package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"strings"
	"sync"

	irc "github.com/fluffle/goirc/client"
	"github.com/mitnk/ircbot/db"
)

func main() {
	var dbname = flag.String("dbname", "ircbot", "PG DB Name")
	var user = flag.String("user", "ircbot", "PG User Name")
	var proxy = flag.String("proxy", "",
		"HTTP or Socks proxy, e.g. socks5://localhost:1080")
	flag.Parse()

	hosts := db.GetHostList(*dbname, *user)
	if len(hosts) == 0 {
		fmt.Printf("no hosts found.\n")
		return
	}

	// spawn group of worker goroutines
	var wg sync.WaitGroup
	for i := 0; i < len(hosts); i++ {
		wg.Add(1)
		host := hosts[i]
		go MonitorIRCHost(host, *proxy, *dbname, *user, wg)
	}
	// wait for the workers to finish
	wg.Wait()
}

func MonitorIRCHost(host db.Host, proxy, dbname, user string, wg sync.WaitGroup) {
	cfg := irc.NewConfig(host.Nick)
	cfg.SSL = host.Ssl
	cfg.SSLConfig = &tls.Config{ServerName: host.Host}
	cfg.Server = fmt.Sprintf("%s:%d", host.Host, host.Port)
	cfg.NewNick = func(n string) string { return n + "^" }
	if len(proxy) > 0 {
		cfg.Proxy = proxy
	}
	c := irc.Client(cfg)

	c.EnableStateTracking()

	// Add handlers to do things here!
	// e.g. join a channel on connect.
	c.HandleFunc(irc.CONNECTED,
		func(conn *irc.Conn, line *irc.Line) {
			fmt.Println("Connected to IRC Server.")
			rooms := db.GetRoomList(host, dbname, user)
			for i := 0; i < len(rooms); i++ {
				name := rooms[i].Name
				if strings.HasPrefix(name, "#") {
					conn.Join(name)
				} else {
					conn.Join(fmt.Sprintf("#%s", name))
				}
				fmt.Printf("Joined room %s\n", name)
			}
		})

	// Set up a handler to notify of disconnect events.
	quit := make(chan bool)
	c.HandleFunc("DISCONNECTED",
		func(conn *irc.Conn, line *irc.Line) {
			wg.Done()
			fmt.Println("Disconnected from IRC Server.")
			quit <- true
		})

	c.HandleFunc(irc.NOTICE,
		func(conn *irc.Conn, line *irc.Line) {
			fmt.Printf("%s %s %s %s\n", line.Nick, line.Cmd, line.Args, line.Time)
		})

	c.HandleFunc(irc.PRIVMSG,
		func(conn *irc.Conn, line *irc.Line) {
			room := line.Args[0]
			msg := line.Args[1]
			db.SaveMessage(host, dbname, user, room, line.Nick, msg, "M", line.Time)
			fmt.Printf("[%s][%s]%s: %s\n",
				line.Time.Format("15:04:05.000"),
				room, line.Nick, msg)
		})
	c.HandleFunc(irc.ACTION,
		func(conn *irc.Conn, line *irc.Line) {
			fmt.Printf("%s %s %s %s\n", line.Nick, line.Cmd, line.Args, line.Time)
			room := line.Args[0]
			msg := line.Args[1]
			db.SaveMessage(host, dbname, user, room, line.Nick, msg, "A", line.Time)
			fmt.Printf("[%s][%s]%s: %s\n", line.Time.Format("15:04:05.000"), room, line.Nick, msg)
		})

	for {
		// connect to server
		if err := c.Connect(); err != nil {
			fmt.Printf("Connection error: %s\n", err)
			return
		}

		// wait on quit channel
		<-quit
	}
}
