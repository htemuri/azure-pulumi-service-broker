package main

import (
	"log"
	"os"

	"github.com/htemuri/azure-pulumi-service-broker/pkg/template"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
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

	// // create the key values for the project

	// _, err = js.CreateKeyValue(&nats.KeyValueConfig{
	// 	Bucket:      "projects",
	// 	Description: "configuration for all projects",
	// })
	// if err != nil {
	// 	logger.Fatal("failed to create kv bucket: ", err)
	// }
	// keys, _ := kv.Keys()
	// logger.Printf("keys available: %v", keys)

	// rev, err := kv.PutString("name1", "hanta2")
	// if err != nil {
	// 	logger.Println("failed to put key 'name' with error: ", err)
	// }
	// logger.Println("Put key 'name' with revision:", rev)

	// assume I unmarshalled grpc request to project protobuf go type
	project := template.Project{
		Name:               "covid",
		Users:              []*template.UserPersonaEntry{{Role: template.RoleType_ROLE_TYPE_ADMIN, Users: []*template.User{{UserPrincipalName: "productivity.catalyst766_slmail.me#EXT#@productivitycatalyst766slma.onmicrosoft.com", ObjectId: "b70d6761-f96e-4c7e-a352-b459099a3c09"}}}, {Role: template.RoleType_ROLE_TYPE_DEVELOPER, Users: []*template.User{{UserPrincipalName: "person1@productivitycatalyst766slma.onmicrosoft.com", ObjectId: "1cf052d6-aeed-4031-8fd3-aa857a3a6b29"}}}, {Role: template.RoleType_ROLE_TYPE_READER, Users: []*template.User{{UserPrincipalName: "person2@productivitycatalyst766slma.onmicrosoft.com", ObjectId: "56752270-5c9a-488f-a398-62e7c3108b30"}}}},
		Groups:             make([]*template.GroupPersonaEntry, 0),
		ServicePrincipal:   &template.ServicePrincipalOptions{Enabled: false},
		StorageAccount:     &template.StorageAccountOptions{Enabled: false},
		KeyVaultOptions:    &template.KeyVaultOptions{Enabled: false},
		DataFactoryOptions: &template.DataFactoryOptions{Enabled: false},
	}

	dataBytes, _ := proto.Marshal(&project)

	// create/update the project stream

	_, err = js.AddStream(&nats.StreamConfig{Name: "ProjectJobQueue", Description: "Stream to manage active jobs for projects", Subjects: []string{"create", "update", "failed"}})
	if err != nil {
		logger.Fatal("failed to create 'ProjectJobQueue' stream with error: ", err)
	}
	_, err = js.Publish("create", dataBytes)
	if err != nil {
		logger.Println("failed to publish to subject 'create' with error: ", err)
	}

}
