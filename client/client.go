package main

import (
	"bufio"
	"context"
	"fmt"
	proto "handin3/grpc"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type ChitChatClient struct {
	client proto.Client
	clk    int32
}

func main() {

	ccclient := &ChitChatClient{}

	ccclient.start_client()
}

func (c *ChitChatClient) start_client() {

	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	//Default values
	username := "John Doe"
	serverAddr := "127.0.0.1"
	serverPort := ":5050"

	c.clk = 0 //Lamport Clock

	if len(os.Args) > 1 {
		username = os.Args[1]
	}
	if len(os.Args) > 2 {
		serverPort = os.Args[2]
	}

	address := serverAddr + serverPort

	//Start up and configure logging output to file
	f, err := os.OpenFile("logs/client"+username+"log"+time.Now().Format("20060102150405")+".log", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		log.Fatal(err)
	}

	//defer to close when we are done with it.
	defer f.Close()

	//set output of logs to f
	log.SetOutput(f)

	opts := grpc.WithTransportCredentials(insecure.NewCredentials())
	conn, err := grpc.NewClient(address, opts)

	if err != nil {
		log.Fatalf("Something went wrong! %s", err.Error())
	}

	proto_client := proto.NewChitChatClient(conn)

	c.client = proto.Client{Uuid: uuid.New().String(), Username: username, Clock: c.clk}

	go c.handle_incoming(proto_client)

	fmt.Println("Connected successfully!")

	c.handle_message(proto_client, cancel)

	defer conn.Close()

}

func (c *ChitChatClient) handle_message(proto_client proto.ChitChatClient, cancel context.CancelFunc) {
	reader := bufio.NewReader(os.Stdin)

	for {
		text, _ := reader.ReadString('\n')
		text = strings.TrimSpace(text)
		if text == `\x` {
			//Disconnect from server
			response, err := proto_client.Unsubscribe(context.Background(), &c.client)
			if response.Result {
				cancel()
				break
			} else {
				log.Printf("Could not unsubscribe! %s \n", err)
			}

		}
		c.clk = c.clk + 1
		message := proto.Message{Uuid: c.client.Uuid, Message: text, Clock: c.clk, Username: c.client.Username, Timestamp: time.Now().Format("02-01-2006 15:04:05")}
		response, err := proto_client.PublishMessage(context.Background(), &message)

		c.clk = max(c.clk, response.Clock) + 1

		if err != nil {
			log.Printf("[LT: %d] Something went wrong! %s \n", c.clk, err)
		}
		if !response.Result {
			log.Printf("[LT: %d] Server did not receive message!", c.clk)
		}
	}

}

func (c *ChitChatClient) handle_incoming(proto_client proto.ChitChatClient) {
	c.clk = c.clk + 1
	stream, err := proto_client.Subscribe(context.Background(), &c.client)

	if err != nil {
		log.Fatalf("[LT: %d] Subscribe failed: %v", c.clk, err)
	} else {
		log.Printf("[LT: %d] Subscribed successfully. Listening for messages...", c.clk)
	}

	for {
		message, err := stream.Recv()
		if err == io.EOF {
			log.Printf("[LT: %d] Server closed stream.", c.clk)
			break
		}
		if err != nil {
			log.Printf("[LT: %d] Error receiving: %v", c.clk, err)
			break
		}

		c.clk = max(c.clk, message.Clock) + 1
		log.Printf("[%s @ %d] %s: %s \n", message.Timestamp, c.clk, message.Username, message.Message)
		fmt.Printf("[%s @ %d] %s: %s \n", message.Timestamp, c.clk, message.Username, message.Message)
	}
}
