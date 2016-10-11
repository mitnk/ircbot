package main

import (
	"bufio"
	"crypto/tls"
	"flag"
	"fmt"
	"os"
	"strings"
	"sync"

	irc "github.com/fluffle/goirc/client"
	"github.com/mitnk/ircbot/db"
)

var G = map[string]map[string]string{"showchat": {}}

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

	// set up a goroutine to read commands from stdin
	in := make(chan string, 4)
	go func() {
		con := bufio.NewReader(os.Stdin)
		for {
			s, err := con.ReadString('\n')
			if err != nil {
				// wha?, maybe ctrl-D...
				close(in)
				break
			}
			// no point in sending empty lines down the channel
			if len(s) > 2 {
				in <- s[0 : len(s)-1]
			}
		}
	}()

	// set up a goroutine to do parsey things with the stuff from stdin
	go func() {
		for cmd := range in {
			if strings.HasPrefix(cmd, ":showchat ") {
				n := strings.Index(cmd, " ")
				room := strings.Trim(cmd[n:], " ")
				if len(room) == 0 || !strings.HasPrefix(room, "#") {
					fmt.Println("invalid room name, should start with #")
					continue
				}
				if G["showchat"][room] != "t" {
					G["showchat"][room] = "t"
				} else {
					G["showchat"][room] = "f"
				}
			}
		}
	}()

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
			fmt.Printf("connected to server %s\n", host.Name)
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
			fmt.Printf("ERROR disconnected from server %s\n", host.Name)
			quit <- true
		})

	c.HandleFunc(irc.NOTICE,
		func(conn *irc.Conn, line *irc.Line) {
			fmt.Printf("[%s] %s %s %s\n",
				line.Time.Format("2006-01-02 15:04:05"),
				line.Nick, line.Cmd, line.Args)
		})

	c.HandleFunc(irc.PRIVMSG,
		func(conn *irc.Conn, line *irc.Line) {
			room := line.Args[0]
			msg := line.Args[1]
			db.SaveMessage(host, dbname, user, room, line.Nick, msg, "M", line.Time)
			if G["showchat"][room] == "t" {
				fmt.Printf("[%s][%s][%s]%s: %s\n",
					line.Time.Format("15:04:05"),
					host.Name, room, line.Nick, msg)
			}
		})
	c.HandleFunc(irc.ACTION,
		func(conn *irc.Conn, line *irc.Line) {
			room := line.Args[0]
			msg := line.Args[1]
			db.SaveMessage(host, dbname, user, room, line.Nick, msg, "A", line.Time)
			if G["showchat"][room] == "t" {
				fmt.Printf("[%s][%s][%s]%s [ACTION] %s\n",
					line.Time.Format("15:04:05"),
					host.Name, room, line.Nick, msg)
			}
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
