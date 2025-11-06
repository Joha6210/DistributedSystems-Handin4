package main

import (
	"context"
	"fmt"
	proto "handin4/grpc"
	"log"
	"net"
	"os"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
)

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

func main() {

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
