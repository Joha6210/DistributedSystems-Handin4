package main

import (
	"context"
	"fmt"
	"log"
	proto "main/grpc"
	"net"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/grandcat/zeroconf"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type RicartArgawalaClient struct {
	nodeId string
	clk    int32
}

type RicartArgawalaServer struct {
	proto.UnimplementedRicartArgawalaServer
	nodeId          string
	clk             int32
	serverPort      int
	wantCS          bool
	requestTS       int32
	replyCount      int
	totalNodes      int
	deferredReplies map[string]bool
	mu              sync.Mutex
}

func main() {
	f, err := os.OpenFile("logs/serverlog"+time.Now().Format("20060102150405")+".log", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		log.Fatal(err)
	}
	log.SetOutput(f)
	defer f.Close()

	if len(os.Args) < 3 {
		log.Fatalf("Usage: go run main.go <NodeID> <Port>")
	}

	nodeId := os.Args[1]
	port, _ := strconv.Atoi(os.Args[2])

	node := &RicartArgawalaClient{nodeId: nodeId, clk: 0}
	server := &RicartArgawalaServer{
		nodeId:          nodeId,
		clk:             0,
		serverPort:      port,
		deferredReplies: make(map[string]bool),
	}

	// Start gRPC server
	go server.start_server()

	// Advertise this node
	go advertiseNode(server.nodeId, server.serverPort)

	// Wait briefly to let others announce
	time.Sleep(3 * time.Second)

	// Discover peers
	peers := node.discoverPeers()
	server.totalNodes = len(peers) + 1

	// Run Ricart–Agrawala
	node.ricartArgawala(peers, server)
}


func (c *RicartArgawalaClient) discoverPeers() []proto.RicartArgawalaClient {

	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	//Start up and configure logging output to file
	f, err := os.OpenFile("logs/client"+c.nodeId+"log"+time.Now().Format("20060102150405")+".log", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		log.Fatal(err)
	}

	//defer to close when we are done with it.
	defer f.Close()

	//set output of logs to f
	log.SetOutput(f)

	discovered := make(chan string)

	discoverNodes(c.nodeId, discovered)

	var peers []proto.RicartArgawalaClient

	for addr := range discovered {
		opts := grpc.WithTransportCredentials(insecure.NewCredentials())
		conn, err := grpc.NewClient(addr, opts)
		if err != nil {
			log.Printf("[%s] Failed to connect to %s: %v", c.nodeId, addr, err)
			continue
		}
		client := proto.NewRicartArgawalaClient(conn)
		peers = append(peers, client)
	}

	log.Printf("[%s] Connected to %d peers", c.nodeId, len(peers))
	return peers

}

func (s *RicartArgawalaServer) Request(ctx context.Context, msg *proto.Message) (*proto.Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.clk = max(s.clk, msg.Clock) + 1
	log.Printf("[%s] Received %s from %s (Clock=%d)", s.nodeId, msg.Content, msg.NodeId, msg.Clock)

	switch msg.Content {
	case "Request":
		// Should we defer or reply immediately?
		deferReply := s.wantCS && ((msg.Clock < s.requestTS) == false && msg.NodeId > s.nodeId)
		if deferReply {
			s.deferredReplies[msg.NodeId] = true
			log.Printf("[%s] Deferred reply to %s", s.nodeId, msg.NodeId)
			return &proto.Message{NodeId: s.nodeId, Clock: s.clk, Content: "Deferred"}, nil
		} else {
			log.Printf("[%s] Sending REPLY to %s", s.nodeId, msg.NodeId)
			return &proto.Message{NodeId: s.nodeId, Clock: s.clk, Content: "Reply"}, nil
		}

	case "Reply":
		s.replyCount++
		log.Printf("[%s] Got reply from %s (%d/%d)", s.nodeId, msg.NodeId, s.replyCount, s.totalNodes-1)

	case "Release":
		log.Printf("[%s] Node %s released CS", s.nodeId, msg.NodeId)
	}

	return &proto.Message{NodeId: s.nodeId, Clock: s.clk, Content: "Ack"}, nil
}

func max(a, b int32) int32 {
	if a > b {
		return a
	}
	return b
}

func (c *RicartArgawalaClient) ricartArgawala(peers []proto.RicartArgawalaClient, s *RicartArgawalaServer) {
	ctx := context.Background()

	s.mu.Lock()
	s.wantCS = true
	s.replyCount = 0
	s.requestTS = s.clk + 1
	s.mu.Unlock()

	log.Printf("[%s] Broadcasting request for CS (Clock=%d)", c.nodeId, s.requestTS)

	for _, peer := range peers {
		go func(p proto.RicartArgawalaClient) {
			resp, err := p.Request(ctx, &proto.Message{
				NodeId:  c.nodeId,
				Clock:   s.requestTS,
				Content: "Request",
			})
			if err != nil {
				log.Printf("[%s] Error sending request: %v", c.nodeId, err)
				return
			}

			if resp.Content == "Reply" {
				s.mu.Lock()
				s.replyCount++
				s.mu.Unlock()
				log.Printf("[%s] Got direct reply from peer", c.nodeId)
			}
		}(peer)
	}

	// Wait until all replies are received
	for {
		s.mu.Lock()
		if s.replyCount >= s.totalNodes-1 {
			s.mu.Unlock()
			break
		}
		s.mu.Unlock()
		time.Sleep(500 * time.Millisecond)
	}

	// Enter critical section
	fmt.Printf("\n[Node %s] ENTERING CRITICAL SECTION at clock %d\n", c.nodeId, s.clk)
	time.Sleep(3 * time.Second)
	fmt.Printf("[Node %s] LEAVING CRITICAL SECTION\n", c.nodeId)

	// Release deferred replies
	s.mu.Lock()
	s.wantCS = false
	for node := range s.deferredReplies {
		go func(target string) {
			// Find the peer client and send the reply
			for _, peer := range peers {
				peer.Request(ctx, &proto.Message{
					NodeId:  c.nodeId,
					Clock:   s.clk,
					Content: "Reply",
				})
			}
			log.Printf("[%s] Sent deferred reply to %s", c.nodeId, target)
		}(node)
	}
	s.deferredReplies = make(map[string]bool)
	s.mu.Unlock()
}

func (s *RicartArgawalaServer) start_server() {
	address := fmt.Sprintf(":%d", s.serverPort)
	grpcServer := grpc.NewServer()
	listener, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatalf("[%s] Could not create server on %s: %v", s.nodeId, address, err)
	}

	proto.RegisterRicartArgawalaServer(grpcServer, s)
	log.Printf("[%s] gRPC server now listening on %s...\n", s.nodeId, address)
	grpcServer.Serve(listener)
}

func advertiseNode(nodeID string, port int) {
	server, err := zeroconf.Register(
		fmt.Sprintf("node-%s", nodeID), // service instance name
		"_ricartagrawala._tcp",         // service type
		"local.",                       // service domain
		port,                           // service port
		[]string{"nodeID=" + nodeID},   // text records
		nil,                            // use system interface
	)
	if err != nil {
		log.Fatalf("Failed to advertise node %s: %v", nodeID, err)
	}
	log.Printf("[%s] Advertised on network (port %d)", nodeID, port)

	// Keep advertising until process exits
	defer server.Shutdown()
	select {}
}

func discoverNodes(nodeID string, discovered chan<- string) {
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		log.Fatalf("[%s] Failed to initialize resolver: %v", nodeID, err)
	}

	entries := make(chan *zeroconf.ServiceEntry)
	go func(results <-chan *zeroconf.ServiceEntry) {
		for entry := range results {
			if entry.Instance != fmt.Sprintf("node-%s", nodeID) {
				address := fmt.Sprintf("%s:%d", entry.AddrIPv4[0].String(), entry.Port)
				log.Printf("[%s] Discovered peer: %s (%s)", nodeID, entry.Instance, address)
				discovered <- address
			}
		}
	}(entries)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err = resolver.Browse(ctx, "_ricartagrawala._tcp", "local.", entries)
	if err != nil {
		log.Fatalf("[%s] Failed to browse: %v", nodeID, err)
	}

	<-ctx.Done() // wait until timeout
	close(discovered)
}
