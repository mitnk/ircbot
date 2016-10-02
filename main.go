package main

import (
	"bufio"
	"crypto/tls"
	"flag"
	"fmt"
	"os"
	"strings"

	irc "github.com/fluffle/goirc/client"
	"github.com/mitnk/ircbot/db"
)

func main() {
	var irchost = flag.String("irchost", "chat.freenode.net",
		"IRC server host")
	var ircport = flag.Int("ircport", 7000, "IRC server port")

	var hostname = flag.String("hostname", "freenode", "irc host name in DB")

	var nick = flag.String("nick", "shuiniu", "IRC nick name")
	var nossl = flag.Bool("nossl", false, "Do not use SSL")
	var proxy = flag.String("proxy", "",
		"HTTP or Socks proxy, e.g. socks5://localhost:1080")
	flag.Parse()

	cfg := irc.NewConfig(*nick)
	cfg.SSL = !(*nossl)
	cfg.SSLConfig = &tls.Config{ServerName: *irchost}
	cfg.Server = fmt.Sprintf("%s:%d", *irchost, *ircport)
	cfg.NewNick = func(n string) string { return n + "^" }
	if len(*proxy) > 0 {
		cfg.Proxy = *proxy
	}
	c := irc.Client(cfg)

	c.EnableStateTracking()
	// Add handlers to do things here!
	// e.g. join a channel on connect.
	c.HandleFunc(irc.CONNECTED,
		func(conn *irc.Conn, line *irc.Line) {
			fmt.Println("Connected to IRC Server.")
			rooms := db.GetRoomList(*hostname)
			for rooms.Next() {
				var id int
				var room_name string
				rooms.Scan(&id, &room_name)
				if strings.HasPrefix(room_name, "#") {
					conn.Join(room_name)
				} else {
					conn.Join(fmt.Sprintf("#%s", room_name))
				}
				fmt.Printf("Joined room %s\n", room_name)
			}
		})

	// Set up a handler to notify of disconnect events.
	quit := make(chan bool)
	c.HandleFunc("DISCONNECTED",
		func(conn *irc.Conn, line *irc.Line) {
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
			db.SaveMessage(*hostname, room, line.Nick, msg, "M", line.Time)
			fmt.Printf("[%s][%s]%s: %s\n",
				line.Time.Format("15:04:05.000"),
				room, line.Nick, msg)
		})
	c.HandleFunc(irc.ACTION,
		func(conn *irc.Conn, line *irc.Line) {
			fmt.Printf("%s %s %s %s\n", line.Nick, line.Cmd, line.Args, line.Time)
			room := line.Args[0]
			msg := line.Args[1]
			db.SaveMessage(*hostname, room, line.Nick, msg, "A", line.Time)
			fmt.Printf("[%s][%s]%s: %s\n", line.Time.Format("15:04:05.000"), room, line.Nick, msg)
		})

	// set up a goroutine to read commands from stdin
	in := make(chan string, 4)
	reallyquit := false
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
			if cmd[0] == ':' {
				switch idx := strings.Index(cmd, " "); {
				case cmd[1] == 'm':
					inner_cmd := cmd[idx+1:]
					inner_idx := strings.Index(inner_cmd, " ")
					if inner_idx == -1 {
						continue
					}
					channel := inner_cmd[:inner_idx]
					msg := inner_cmd[inner_idx+1:]
					c.Privmsg(channel, msg)
				case idx == -1:
					continue
				case cmd[1] == 'q':
					reallyquit = true
					c.Quit(cmd[idx+1 : len(cmd)])
				case cmd[1] == 's':
					reallyquit = true
					c.Close()
				case cmd[1] == 'j':
					c.Join(cmd[idx+1 : len(cmd)])
				}
			} else {
				c.Raw(cmd)
			}
		}
	}()

	for !reallyquit {
		// connect to server
		if err := c.Connect(); err != nil {
			fmt.Printf("Connection error: %s\n", err)
			return
		}

		// wait on quit channel
		<-quit
	}
}
