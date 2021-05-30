package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

var clients = make(map[string]*Client, 0)

type Client struct {
	Id       string
	Username string
	conn     net.Conn
}

func NewClient(conn net.Conn, username string) *Client {
	return &Client{
		Id:       strconv.Itoa(int(uuid.New().ID())),
		Username: username,
		conn:     conn,
	}
}

func (c *Client) HandleEvents() error {

	packet := make(map[string]string, 0)

	for {

		data, err := wsutil.ReadClientText(c.conn)
		if err != nil {
			return err
		}

		if err := json.Unmarshal(data, &packet); err != nil {
			log.Println(err)
			continue
		}

		switch packet["command"] {
		case "Calling", "Answered":
			if to := packet["to"]; to != c.Username {
				if toConn, ok := clients[to]; ok {
					if err := wsutil.WriteServerMessage(toConn.conn, ws.OpText, data); err != nil {
						log.Println(err)
					}
				}
			}
			break
		}
	}

}

func main() {

	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", "0.0.0.0", 3000))
	if err != nil {
		log.Fatal(err)
	}

	defer listener.Close()

	router := mux.NewRouter()
	router.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {

		conn, _, _, err := ws.UpgradeHTTP(req, w)
		if err != nil {
			log.Fatal(err)
			return
		}

		username := req.URL.Query().Get("u")

		log.Println(fmt.Sprintf("New client joined! [%s]", username))

		client := NewClient(conn, username)
		clients[username] = client
		client.HandleEvents()

	})

	log.Println("Websocket server running...")
	log.Printf("http_err: %v", http.Serve(listener, router))
}
