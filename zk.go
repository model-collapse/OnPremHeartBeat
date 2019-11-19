package main

import (
	"log"
	"time"

	"github.com/samuel/go-zookeeper/zk"
)

var ZKConn *zk.Conn

func InitializeZK(servers []string) {
	zc, evt, err := zk.Connect(servers, 10*time.Second)
	ZKConn = zc
	if err != nil {
		log.Fatalf("Fail to connect to ZK! %v", err)
	}

	go func() {
		for evt := range evt {
			if evt.Err != nil {
				log.Printf("[ERR ZK] %v", evt.Err)
			}
		}
	}()

	return
}
