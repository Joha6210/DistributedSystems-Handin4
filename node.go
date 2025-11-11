package main

import (
	"context"
	"fmt"
	"log"
	proto "main/grpc"
	"math/rand"
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
	peers  map[string]proto.RicartArgawalaClient
	mu     sync.Mutex
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

	nodeId := os.Args[1]
	port, _ := strconv.Atoi(os.Args[2])

	f, err := os.OpenFile("logs/node"+nodeId+"_"+time.Now().Format("20060102150405")+".log", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		log.Fatal(err)
	}
	log.SetOutput(f)
	defer f.Close()

	if len(os.Args) < 3 {
		log.Fatalf("Usage: go run main.go <NodeID> <Port>")
	}

	node := &RicartArgawalaClient{nodeId: nodeId, peers: make(map[string]proto.RicartArgawalaClient)}
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
	node.startPeerDiscovery()

	// Run Ricart–Agrawala
	node.ricartArgawala(server)
}

func (c *RicartArgawalaClient) startPeerDiscovery() {
	go func() {
		for {
			discovered := make(chan string)
			go discoverNodes(c.nodeId, discovered)

			for addr := range discovered {
				c.mu.Lock()
				if _, exists := c.peers[addr]; !exists {
					opts := grpc.WithTransportCredentials(insecure.NewCredentials())
					conn, err := grpc.NewClient(addr, opts)
					if err != nil {
						log.Printf("[Node %s] Failed to connect to %s: %v", c.nodeId, addr, err)
						c.mu.Unlock()
						continue
					}
					client := proto.NewRicartArgawalaClient(conn)
					c.peers[addr] = client
					log.Printf("[Node %s] Added new peer: %s", c.nodeId, addr)
				}
				c.mu.Unlock()
			}

			time.Sleep(5 * time.Second) // adjust interval
		}
	}()
}

func (s *RicartArgawalaServer) Request(ctx context.Context, msg *proto.Message) (*proto.Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.clk = max(s.clk, msg.Clock) + 1
	fmt.Printf("[Node %s] Received %s from %s (Clock=%d)\n", s.nodeId, msg.Content, msg.NodeId, msg.Clock)
	log.Printf("[Node %s] Received %s from %s (Clock=%d)", s.nodeId, msg.Content, msg.NodeId, msg.Clock)

	switch msg.Content {
	case "Request":
		// Should we defer or reply immediately?
		deferReply := s.wantCS && (s.requestTS < msg.Clock || (s.requestTS == msg.Clock && s.nodeId < msg.NodeId))
		if deferReply {
			s.deferredReplies[msg.NodeId] = true
			fmt.Printf("[Node %s] Deferred reply to %s\n", s.nodeId, msg.NodeId)
			log.Printf("[Node %s] Deferred reply to %s", s.nodeId, msg.NodeId)
		} else {
			fmt.Printf("[Node %s] Sending REPLY to %s\n", s.nodeId, msg.NodeId)
			log.Printf("[Node %s] Sending REPLY to %s", s.nodeId, msg.NodeId)
			return &proto.Message{NodeId: s.nodeId, Clock: s.clk, Content: "Reply"}, nil
		}

	case "Reply":
		s.replyCount++
		fmt.Printf("[Node %s] Got reply from %s (%d/%d)\n", s.nodeId, msg.NodeId, s.replyCount, s.totalNodes-1)
		log.Printf("[Node %s] Got reply from %s (%d/%d)", s.nodeId, msg.NodeId, s.replyCount, s.totalNodes-1)
	}

	return &proto.Message{NodeId: s.nodeId, Clock: s.clk, Content: "Ack"}, nil
}

func max(a, b int32) int32 {
	if a > b {
		return a
	}
	return b
}

func (c *RicartArgawalaClient) ricartArgawala(s *RicartArgawalaServer) {
	ctx := context.Background()

	for {
		s.mu.Lock()
		// Randomly decide whether to enter CS
		if rand.Float32() < 0.5 { // 50% chance to skip
			fmt.Printf("[Node %s] Decided not to enter CS this time \n", c.nodeId)
			log.Printf("[Node %s] Decided not to enter CS this time", c.nodeId)
			s.mu.Unlock()
			time.Sleep(time.Duration(rand.Intn(3000)+1000) * time.Millisecond) // 1-4 seconds
			continue
		}
		s.totalNodes = len(c.peers)
		s.wantCS = true
		s.replyCount = 0
		s.requestTS = s.clk + 1
		s.mu.Unlock()

		log.Printf("[%s] Broadcasting request for CS (Clock=%d)", c.nodeId, s.requestTS)
		c.mu.Lock()
		peers := make([]proto.RicartArgawalaClient, 0, len(c.peers))
		for _, p := range c.peers {
			peers = append(peers, p)
		}

		s.mu.Lock()
		s.clk++
		s.mu.Unlock()
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
					s.clk = max(s.clk, resp.Clock) + 1
					s.replyCount++
					s.mu.Unlock()
					log.Printf("[%s] Got reply from %s (%d/%d)", s.nodeId, resp.NodeId, s.replyCount, s.totalNodes-1)
				}
			}(peer)
		}
		c.mu.Unlock()
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
		log.Printf("[Node %s] ENTERING CRITICAL SECTION at clock %d", c.nodeId, s.clk)
		fmt.Printf("\n[Node %s] ENTERING CRITICAL SECTION at clock %d\n", c.nodeId, s.clk)
		s.mu.Lock()
		s.clk++
		s.mu.Unlock()
		time.Sleep(3 * time.Second)
		fmt.Printf("[Node %s] LEAVING CRITICAL SECTION at clock %d\n", c.nodeId, s.clk)
		log.Printf("[Node %s] LEAVING CRITICAL SECTION at clock %d", c.nodeId, s.clk)

		// Release deferred replies
		s.mu.Lock()
		c.mu.Lock()
		s.wantCS = false
		s.clk++ //Lamport clock sending request
		for node := range s.deferredReplies {
			go func(target string) {
				c.mu.Lock()
				defer c.mu.Unlock()
				if peer, ok := c.peers[target]; ok {
					peer.Request(ctx, &proto.Message{
						NodeId:  c.nodeId,
						Clock:   s.clk,
						Content: "Reply",
					})
					log.Printf("[%s] Sent deferred reply to %s", c.nodeId, target)
				}
			}(node)
		}
		s.deferredReplies = make(map[string]bool)
		c.mu.Unlock()
		s.mu.Unlock()

		// Random wait before next CS attempt
		time.Sleep(time.Duration(rand.Intn(4000)+1000) * time.Millisecond) // 1-5 seconds
	}
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
	fmt.Sprintf("[%s] Advertised on network (port %d)", nodeID, port)
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
