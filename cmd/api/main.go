package main

import (
	"log"
	"os"

	"github.com/nats-io/nats.go"
)

func main() {
	logger := log.New(os.Stdout, "[Broker API]: ", log.Ldate|log.Ltime|log.Lmsgprefix)
	natsServer := nats.DefaultURL

	logger.Println("initializing connection to nats server: ", natsServer)
	nc, err := nats.Connect(natsServer)
	if err != nil {
		logger.Fatal("failed to connect to nats server: ", err)
	}
	defer nc.Close()

	logger.Println("upgrading nats connection to jetstream")
	js, err := nc.JetStream()
	if err != nil {
		logger.Fatal("failed to upgrade nats connection to jetstream: ", err)
	}
	logger.Println("upgraded connection to jetstream")

	kv, err := js.CreateKeyValue(&nats.KeyValueConfig{
		Bucket:      "hanta",
		Description: "test bucket for project hanta",
	})
	if err != nil {
		logger.Fatal("failed to create kv bucket: ", err)
	}
	keys, _ := kv.Keys()
	logger.Printf("keys available: %v", keys)
	// rev, err := kv.PutString("name1", "hanta2")
	// if err != nil {
	// 	logger.Println("failed to put key 'name' with error: ", err)
	// }
	// logger.Println("Put key 'name' with revision:", rev)
}
