package main

import (
	"context"
	"fmt"
	"log"
	proto "main/grpc"
	"net"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type RicartArgawalaClient struct {
	nodeId string
	clk    int32
}

type RicartArgawalaServer struct {
	proto.UnimplementedRicartArgawalaServer
	clk int32
}

func main() {

	//Start up and configure logging output to file
	f, err := os.OpenFile("logs/serverlog"+time.Now().Format("20060102150405")+".log", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		log.Fatal(err)
	}

	//defer to close when we are done with it.
	defer f.Close()

	node := &RicartArgawalaClient{}

	node.start_client()

	//set output of logs to f
	log.SetOutput(f)

	server := &RicartArgawalaServer{}

	server.start_server()
}

func (c *RicartArgawalaClient) start_client() {

	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	//Default values
	c.nodeId = "Node1"
	serverAddr := "127.0.0.1"
	serverPort := ":5050"

	c.clk = 0 //Lamport Clock

	if len(os.Args) > 1 {
		c.nodeId = os.Args[1]
	}
	if len(os.Args) > 2 {
		serverPort = os.Args[2]
	}

	address := serverAddr + serverPort

	//Start up and configure logging output to file
	f, err := os.OpenFile("logs/client"+c.nodeId+"log"+time.Now().Format("20060102150405")+".log", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
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

	proto_client := proto.NewRicartArgawalaClient(conn)
	fmt.Println("Connected successfully!")

	c.ricartArgawala(proto_client)

	defer conn.Close()

}

func (c *RicartArgawalaClient) ricartArgawala(proto_client proto.RicartArgawalaClient) {
	c.clk = c.clk + 1
	proto_client.Request(context.Background(), &proto.Message{NodeId: c.nodeId, Clock: c.clk, Content: "Request"});
	//-----------------------Critical Section--------------------
	fmt.Printf("Node: %s entered the Critical section at time: %d", c.nodeId, c.clk)
	//-----------------------End of Critical Section-------------
	c.clk = max(c.clk, message.Clock) + 1
	log.Printf("[%s @ %d] %s: %s \n", message.Timestamp, c.clk, message.Username, message.Message)
	fmt.Printf("[%s @ %d] %s: %s \n", message.Timestamp, c.clk, message.Username, message.Message)
}

func (s *RicartArgawalaServer) start_server() {
	port := ":5050"
	grpcServer := grpc.NewServer()
	listener, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("Could not create server due to: %v", err)
	}

	proto.RegisterRicartArgawalaServer(grpcServer, s)
	log.Printf("gRPC server now listening on %s...\n", port)
	grpcServer.Serve(listener)

}
