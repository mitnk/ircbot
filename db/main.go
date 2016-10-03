package db

import (
	"database/sql"
	"fmt"
	_ "github.com/lib/pq"
	"log"
	"strings"
	"time"
)

type Host struct {
	Name string
	Host string
	Port int
	Ssl  bool
	Nick string
}

func getConnectionString(dbname, user string) string {
	return fmt.Sprintf("dbname=%s user=%s sslmode=disable",
		dbname, user)
}

func GetHostList(dbname, user string) []Host {
	result := make([]Host, 0)
	hosts := DBGetHostList(dbname, user)
	for hosts.Next() {
		var name string
		var host string
		var port int
		var ssl bool
		var nick string
		hosts.Scan(&name, &host, &port, &ssl, &nick)
		result = append(result, Host{name, host, port, ssl, nick})
	}
	return result
}

func DBGetHostList(dbname, user string) *sql.Rows {
	conn_str := getConnectionString(dbname, user)
	db, err := sql.Open("postgres", conn_str)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	results, err := db.Query("select name, host, port, ssl, nick from irc_host")
	if err != nil {
		log.Fatal(err)
	}
	return results
}

func GetRoomList(host Host, dbname, user string) *sql.Rows {
	conn_str := getConnectionString(dbname, user)
	db, err := sql.Open("postgres", conn_str)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	results, err := db.Query("select r.id, r.name "+
		"from irc_room r join irc_host h "+
		"on r.host_id = h.id where h.name = $1",
		host.Name)
	if err != nil {
		log.Fatal(err)
	}
	// defer results.Close()
	return results
}

func getRoomId(db *sql.DB, host, room string) int {
	query := "SELECT r.id FROM " +
		"irc_room r JOIN irc_host h ON r.host_id = h.id " +
		"WHERE h.name = $1 and r.name = $2"
	results, err := db.Query(query, host, room)
	if err != nil {
		log.Fatal(err)
	}
	for results.Next() {
		var id int
		results.Scan(&id)
		return id
	}
	defer results.Close()
	return 0
}

func SaveMessage(host Host, dbname, user, room, nick, msg, typ string, ts time.Time) {
	conn_str := getConnectionString(dbname, user)
	db, err := sql.Open("postgres", conn_str)
	if err != nil {
		log.Fatal(err)
	}

	if !strings.HasPrefix(room, "##") {
		room = strings.Trim(room, "#")
	}
	defer db.Close()
	room_id := getRoomId(db, host.Name, room)
	db.Exec(
		"INSERT INTO irc_message "+
			"(nick, msg, added, room_id, typ) "+
			"values($1, $2, $3, $4, $5)",
		nick, msg, ts, room_id, typ)
	if err != nil {
		log.Fatal(err)
	}
}
