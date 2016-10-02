package db

import (
	"database/sql"
	_ "github.com/lib/pq"
	"log"
	"strings"
	"time"
)

func GetRoomList(host string) *sql.Rows {
	db, err := sql.Open("postgres", "user=mitnk dbname=djapps sslmode=disable")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	results, err := db.Query("select r.id, r.name "+
		"from irc_room r join irc_host h "+
		"on r.host_id = h.id where h.name = $1",
		host)
	if err != nil {
		log.Fatal(err)
	}
	// defer results.Close()
	return results
}

func GetRoomId(host, room string) int {
	query := "SELECT r.id FROM " +
		"irc_room r JOIN irc_host h ON r.host_id = h.id " +
		"WHERE h.name = $1 and r.name = $2"
	db, err := sql.Open("postgres", "user=mitnk dbname=djapps sslmode=disable")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	results, err := db.Query(query, host, room)
	for results.Next() {
		var id int
		results.Scan(&id)
		return id
	}
	defer results.Close()
	return 0
}

func SaveMessage(host, room, nick, msg, typ string, ts time.Time) {
	if !strings.HasPrefix(room, "##") {
		room = strings.Trim(room, "#")
	}
	db, err := sql.Open("postgres", "user=mitnk dbname=djapps sslmode=disable")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	room_id := GetRoomId(host, room)
	db.Exec(
		"INSERT INTO irc_message (nick, msg, added, room_id, typ) values($1, $2, $3, $4, $5)",
		nick, msg, ts, room_id, typ)
	if err != nil {
		log.Fatal(err)
	}
}
