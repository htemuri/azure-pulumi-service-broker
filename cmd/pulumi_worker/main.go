package main

import (
	"context"
	"log"
	"os"
	"sync"

	"github.com/htemuri/azure-pulumi-service-broker/pkg/broker"
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
		logger.Fatal("error loading .env file:", err)
	}

	// TODO: Make this better - maybe add defaults as a method of the config class
	config := Config{
		ProjectStreamName: os.Getenv("PROJECT_STREAM_NAME"),
	}
	if config.ProjectStreamName == "" {
		config.ProjectStreamName = "ProjectJobQueue"
	}

	logger.Println("initializing connection to nats server:", natsServer)
	nc, err := nats.Connect(natsServer)
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

	consumer, err := js.CreateOrUpdateConsumer(ctx, config.ProjectStreamName, jetstream.ConsumerConfig{
		Name: "pulumi_worker", Durable: "pulumi_worker",
	})
	if err != nil {
		logger.Fatalf("failed to create/update durable consumer against %s stream with error: %s\n", config.ProjectStreamName, err)
	}

	if _, err := consumer.Consume(func(msg jetstream.Msg) {
		if msg.Subject() == "failed" || msg.Subject() == "success" {
			return
		}
		msg.InProgress()
		var createProjectRequest broker.CreateProjectRequest
		err = proto.Unmarshal(msg.Data(), &createProjectRequest)
		if err != nil {
			logger.Printf("failed to unmarshal message from subject '%s' with error: \n%s", msg.Subject(), err)
			msg.Nak()
			return
		}
		project := createProjectRequest.GetProject()
		nh := NewNatsHandler(context.Background(), project, createProjectRequest.GetTemplates())
		msg.Ack()
		logger.Printf("received a message from subject '%s' about project with name '%s'\n", msg.Subject(), project.Name)

		templateResponses, err := nh.Handle()
		logger.Printf("passed handle")
		if err != nil {
			logger.Printf("failed to deploy templates for project %s with error: %s", project.Name, err)
			logger.Printf("sending project %s to failed deployment queue\n", project.Name)
			_, err = js.Publish(ctx, "failed", msg.Data())
			if err != nil {
				logger.Printf("failed to send project job to 'failed' subject in nats with error:\n\t%s", err)
			}
			return
		}
		createProjectResponse := &broker.CreateProjectResponse{
			Name:              project.Name,
			TemplateResponses: templateResponses,
		}

		// send to nats successful subject
		bytes, err := proto.Marshal(createProjectResponse)
		if err != nil {
			logger.Printf("failed to marshal project response: %s\n", err)
			return
		}
		_, err = js.Publish(ctx, "success", bytes)
		if err != nil {
			logger.Printf("failed to publish response to nats success subject: %s\n", err)
			return
		}
		logger.Printf("success")
	}); err != nil {
		logger.Fatal("failed to consume messages from durable stream with error:", err)
	}

	// keep waiting for nats messages
	wg.Wait()

	// ctx := context.Background()
	// s, err := auto.UpsertRemoteStackGitSource(ctx, "htemuri/test-templating/git-auto", auto.GitRepo{
	// 	URL:         "https://github.com/pulumi/examples.git",
	// 	Branch:      "master",
	// 	ProjectPath: "aws-go-s3-folder",
	// })
	// if err != nil {
	// 	fmt.Println(err.Error())
	// }
	// _, err = s.Preview(ctx)
	// if err != nil {
	// 	fmt.Println(err.Error())
	// }

}
