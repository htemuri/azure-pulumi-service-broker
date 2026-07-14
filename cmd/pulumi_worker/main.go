package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/htemuri/azure-pulumi-service-broker/pkg/broker"
	"github.com/htemuri/azure-pulumi-service-broker/pkg/templates"
	"github.com/joho/godotenv"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"google.golang.org/protobuf/proto"
)

var logger = log.New(os.Stdout, "[Pulumi Worker]: ", log.Ldate|log.Ltime|log.Lmsgprefix)

func main() {
	natsServer := nats.DefaultURL
	ctx := context.Background()

	runtime := os.Getenv("RUNTIME")
	if runtime != "docker" {
		err := godotenv.Load()
		if err != nil {
			logger.Fatal("error loading .env file:", err)
		}
	}

	if os.Getenv("NATS_SERVER") != "" {
		natsServer = os.Getenv("NATS_SERVER")
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

	kvStore, err := js.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket:      "deployments",
		Description: "List of project deployments in k:v format <deploymentId>:<x GetProjectStatusResponse>",
	})
	if err != nil {
		logger.Fatalf("failed to create/update nats project deployment kv store with error: %s\n", err)
	}

	if _, err := consumer.Consume(func(msg jetstream.Msg) {
		splitSubject := strings.Split(msg.Subject(), ".")
		if len(splitSubject) != 2 {
			logger.Printf("invalid message subject syntax %s. project queue subjects should be in format '<action>.<deploymentId>'", msg.Subject())
			msg.Ack()
			return
		}
		action, deploymentId := splitSubject[0], splitSubject[1]
		if action != "create" {
			msg.Ack()
			return
		}
		var projectStatus broker.GetProjectStatusResponse
		projectStatus.Status = templates.Status_STATUS_IN_PROGRESS
		err = updateProjectStatusKV(ctx, kvStore, deploymentId, &projectStatus)
		if err != nil {
			logger.Print(err)
			msg.Ack()
			return
		}
		msg.InProgress()

		var createProjectRequest broker.CreateProjectRequest
		err = proto.Unmarshal(msg.Data(), &createProjectRequest)
		if err != nil {
			logger.Printf("failed to unmarshal message from subject '%s' with error: \n%s", msg.Subject(), err)
			msg.Ack()
			projectStatus.Status = templates.Status_STATUS_ERROR
			projectStatus.Error = fmt.Sprintf("failed to unmarshal message from subject '%s' with error: \n%s", msg.Subject(), err)
			err = updateProjectStatusKV(ctx, kvStore, deploymentId, &projectStatus)
			if err != nil {
				logger.Print(err)
				return
			}
			return
		}
		project := createProjectRequest.GetProject()
		projectStatus.ProjectName = project.Name
		nh := NewNatsHandler(context.Background(), project, createProjectRequest.GetTemplateRequests())
		msg.Ack()
		logger.Printf("received a message from subject '%s' about project with name '%s'\n", msg.Subject(), project.Name)

		templateResponses, err := nh.Handle()
		if err != nil {
			logger.Printf("failed to deploy templates for project %s with error: %s", project.Name, err)
			projectStatus.Status = templates.Status_STATUS_ERROR
			projectStatus.Error = fmt.Sprintf("failed to deploy templates for project %s with error: %s", project.Name, err)
			err = updateProjectStatusKV(ctx, kvStore, deploymentId, &projectStatus)
			if err != nil {
				logger.Print(err)
				return
			}
			return
		}
		projectStatus.Status = templates.Status_STATUS_SUCCESS
		projectStatus.TemplateResponses = templateResponses
		err = updateProjectStatusKV(ctx, kvStore, deploymentId, &projectStatus)
		if err != nil {
			logger.Print(err)
			return
		}
		logger.Printf("successfully deployed project %s\n", project.Name)
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
