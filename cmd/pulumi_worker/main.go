package main

import (
	"context"
	"log"
	"os"
	"sync"

	"github.com/htemuri/azure-pulumi-service-broker/pkg/template"
	"github.com/joho/godotenv"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"google.golang.org/protobuf/proto"
)

var logger = log.New(os.Stdout, "[Pulumi Worker]: ", log.Ldate|log.Ltime|log.Lmsgprefix)

func main() {
	natsServer := nats.DefaultURL
	ctx := context.Background()

	err := godotenv.Load()
	if err != nil {
		logger.Fatal("Error loading .env file:", err)
	}

	globalVars := Config{
		PulumiClientId:                 os.Getenv("PULUMI_SP_CLIENT_ID"),
		PulumiClientSecret:             os.Getenv("PULUMI_SP_CLIENT_SECRET"),
		TenantId:                       os.Getenv("TENANT_ID"),
		BillingScope:                   os.Getenv("BILLING_SCOPE"),
		ClientProjectManagementGroupId: os.Getenv("CLIENT_PROJ_MGMT_GROUP_ID"),
		Region:                         os.Getenv("REGION"),
		ClientDevVnetIpAllocId:         os.Getenv("CLIENT_DEV_IPAM_RESOURCE_ID"),
		EntraIdAdminObjectIds:          []string{"b70d6761-f96e-4c7e-a352-b459099a3c09"}, // can point to a database list of admins
	}

	logger.Println("initializing connection to nats server:", natsServer)
	nc, err := nats.Connect(nats.DefaultURL)
	if err != nil {
		logger.Fatal("failed to connect to nats server:", err)
	}
	defer nc.Close()

	logger.Println("upgrading nats connection to jetstream")
	js, err := jetstream.New(nc)
	if err != nil {
		logger.Fatal("failed to upgrade nats connection to jetstream: ", err)
	}
	logger.Println("upgraded connection to jetstream")

	wg := sync.WaitGroup{}
	wg.Add(1)

	consumer, err := js.CreateOrUpdateConsumer(ctx, "ProjectJobQueue", jetstream.ConsumerConfig{
		Name: "pulumi_worker", Durable: "pulumi_worker",
	})
	if err != nil {
		logger.Fatal("failed to create/update durable consumer against project job queue stream with error:", err)
	}

	if _, err := consumer.Consume(func(msg jetstream.Msg) {
		if msg.Subject() == "failed" {
			return
		}
		msg.InProgress()
		var project template.Project
		err = proto.Unmarshal(msg.Data(), &project)
		if err != nil {
			logger.Printf("failed to unmarshal message from subject '%s' with error: \n%s", msg.Subject(), err)
			msg.Nak()
			return
		}
		msg.Ack()
		logger.Printf("received a message from subject '%s' about project with name '%s'\n", msg.Subject(), project.Name)
		err = handleProjectEntraUpdate(globalVars, &project)
		if err != nil {
			logger.Printf("failed to update project entra objects:\n\t%s", err)
			_, errPub := js.Publish(ctx, "failed", msg.Data())
			if errPub != nil {
				logger.Printf("failed to send project job to 'failed' subject in nats with error:\n\t%s", errPub)
			}
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			err = handleProjectResourceUpdate(DEV, globalVars, &project)
			if err != nil {
				logger.Printf("failed to provision dev resources:\n\t%s", err)
				_, errPub := js.Publish(ctx, "failed", msg.Data())
				if errPub != nil {
					logger.Printf("failed to send project job to 'failed' subject in nats with error:\n\t%s", errPub)
				}
			}
		}()
		// wg.Add(1)
		// go func() {
		// 	defer wg.Done()
		// 	err = handleProjectUpdate(PRODUCTION, globalVars, &project)
		// 	if err != nil {
		// 		logger.Printf("failed to provision production resources:\n\t%s", err)
		// 		_, errPub := js.Publish(ctx, "failed", msg.Data())
		// 		if errPub != nil {
		// 			logger.Printf("failed to send project job to 'failed' subject in nats with error:\n\t%s", errPub)
		// 		}
		// 	}
		// }()

	}); err != nil {
		logger.Fatal("failed to consume messages from durable stream with error:", err)
	}

	// keep waiting for nats messages
	wg.Wait()

}
