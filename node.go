package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"net"
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

	//Start up and configure logging output to file
	f, err := os.OpenFile("logs/serverlog"+time.Now().Format("20060102150405")+".log", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		log.Fatal(err)
	}

	//defer to close when we are done with it.
	defer f.Close()

	//set output of logs to f
	log.SetOutput(f)

	server := &ChitChatServer{}

	server.start_server()
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

type ChitChatServer struct {
	proto.UnimplementedChitChatServer
	port           string
	messageHistory []*proto.Message
	clients        map[string]*proto.Client
	msgChan        chan *proto.Message
	clientChans    map[string]chan *proto.Message
	uuid           string
	clk            int32
	name           string
}

func (s *ChitChatServer) start_server() {
	s.port = ":5050"
	grpcServer := grpc.NewServer()
	listener, err := net.Listen("tcp", s.port)
	if err != nil {
		log.Fatalf("Could not create server due to: %v", err)
	}

	s.clientChans = make(map[string]chan *proto.Message)
	s.clients = make(map[string]*proto.Client)
	s.msgChan = make(chan *proto.Message, 100)
	s.uuid = uuid.New().String()
	s.name = "Server"
	s.clk = 0

	s.clients[s.uuid] = &proto.Client{Username: s.name}

	proto.RegisterChitChatServer(grpcServer, s)
	log.Printf("gRPC server now listening on %s... at logical time: %d \n", s.port, s.clk)
	grpcServer.Serve(listener)

}

func (s *ChitChatServer) Subscribe(client *proto.Client, stream grpc.ServerStreamingServer[proto.Message]) error {

	s.clk = max(client.Clock, s.clk) + 1
	oldClk := s.clk

	s.clk = s.clk + 1 //Accepts the subscription for the client
	log.Printf("Participant %s joined Chit Chat at logical time %d", client.Username, oldClk)
	msg := proto.Message{Uuid: s.uuid, Message: fmt.Sprintf("Participant %s joined Chit Chat at logical time %d", client.Username, oldClk), Username: s.name, Clock: s.clk}
	s.PublishMessage(context.Background(), &msg)

	s.clients[client.Uuid] = client
	ch := make(chan *proto.Message, 10)
	s.clientChans[client.Uuid] = ch
	defer delete(s.clientChans, client.Uuid)
	defer delete(s.clients, client.Uuid)

	// Send chat history
	for _, msg := range s.messageHistory {
		if err := stream.Send(msg); err != nil {
			log.Printf("Error sending history to %s (%s) at logical time: %d, %s", client.Username, client.Uuid, s.clk, err)
			return err
		}
	}

	// Streaming loop
	for {
		select {
		case msg, leave := <-ch:
			if !leave {
				log.Printf("Stream closed for %s (%s) at logical time: %d", client.Username, client.Uuid, s.clk)
				return nil
			}
			stream.Send(msg)
		case <-stream.Context().Done():
			log.Printf("Client disconnected: %s (%s) at logical time: %d", client.Username, client.Uuid, s.clk)
			return nil
		}
	}
}

func (s *ChitChatServer) PublishMessage(ctx context.Context, message *proto.Message) (*proto.Response, error) {
	result := false
	_, ok := s.clients[message.Uuid]
	if ok {
		s.clk = max(s.clk, message.Clock) + 1 //Receive and  update the lamport clock and increase by one.
		fmt.Printf("[%s @ %d] %s: %s \n", message.Timestamp, s.clk, message.Username, message.Message)
		log.Printf("[%s @ %d] %s: %s \n", message.Timestamp, s.clk, message.Username, message.Message)

		s.messageHistory = append(s.messageHistory, message) //Save the new message to the history
		s.clk++                                              //Local send event
		message.Clock = s.clk
		for _, ch := range s.clientChans {
			ch <- message
		}

		result = true
	}

	if message.Uuid != s.uuid { //Do not increment if the server is the sender
		s.clk++
	}

	return &proto.Response{
		Result: result,
		Clock:  s.clk,
	}, nil
}

func (s *ChitChatServer) Unsubscribe(ctx context.Context, client *proto.Client) (*proto.Response, error) {
	var result bool = false
	ch, leave := s.clientChans[client.Uuid]
	if leave {
		log.Printf("Participant %s left Chit Chat at logical time %d", client.Username, s.clk)
		oldClk := s.clk
		s.clk = max(s.clk, client.Clock) + 1
		msg := proto.Message{Uuid: s.uuid, Message: fmt.Sprintf("Participant %s left Chit Chat at logical time %d", client.Username, oldClk), Username: s.name, Clock: int32(s.clk)}
		s.PublishMessage(context.Background(), &msg)
		close(ch) // this will make the streaming loop in Subscribe exit
		delete(s.clientChans, client.Uuid)
		delete(s.clients, client.Uuid)

		result = true

	}
	s.clk++
	return &proto.Response{
		Result: result,
		Clock:  int32(s.clk),
	}, nil

}
