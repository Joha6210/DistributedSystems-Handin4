# Ricart-Argawala in Golang 
**Author(s):** Johannes Jensen  
**Course:** Distributed Systems — Handin 4  
**Date:** 12-11-2025
**Repo:** [Github repository](https://github.com/Joha6210/DistributedSystems-Handin4)

## Table of contents
- System requirements
- Discussion of Ricart-Argawala implementation
- Appendix

## System requirements
Context, motivation, and high-level problem statement. Keep scope and audience explicit.

This project aims to implement an version of the Ricart-Argawala algorithm to uphold mutual exclusion in a distributed system in golang.

The implementation should meet the requirements stated in the description: [Mandatory Activity 4](https://learnit.itu.dk/mod/assign/view.php?id=235143), R1, R2 and R3.

### R1 (Spec)

The node.go implements a "node" that acts as both a server and a client (in gRPC terms).

The main responsibility of the server is to listen for incoming request from other nodes (peers) and decide if a request should be deferred or acted upon immediately (by sending a reply back, to the requesting node). If it receives a reply from another node it notifies the client by incrementing a `replyCount` counter.

The clients responsibility is to enter the critical section (CS) when allowed to by the other peers. When the node is first started it will wait and listen for other peers before trying to enter the CS. There is a 30% chance for the node not to want to enter the CS, this is done to simulate that all processes does not want to enter the CS at all times.

The implementation of Ricart-Agrawala is loosely based upon the slides [Coordination & Agreement](https://learnit.itu.dk/pluginfile.php/394900/course/section/165227/Coordination_and_agreement.pdf?time=1761814143344) (pdf slide 15), variables names not the same etc.

The CS is simulated by a print statement that is surrounded by log and print statements:

```golang
  // Enter critical section
  log.Printf("[Node %s] ENTERING CRITICAL SECTION at clock %d", c.nodeId, s.clk)
  fmt.Printf("\n[Node %s] ENTERING CRITICAL SECTION at clock %d\n", c.nodeId, s.clk)
  s.mu.Lock()
  s.clk++
  s.mu.Unlock()
  time.Sleep(3 * time.Second)
  fmt.Printf("[Node %s] LEAVING CRITICAL SECTION at clock %d\n", c.nodeId, s.clk)
  log.Printf("[Node %s] LEAVING CRITICAL SECTION at clock %d", c.nodeId, s.clk)
```

### R2 (Safety)

The Ricart-Agrawala algorithm makes sure that only one process (or in this instance node) enters the CS at a time.
This is done by having each node that is requesting/wanting to enter the CS, wait for all nodes(peers) in the network, to allow it, by replying to the request. Each node that the request was sent to will decide if the requesting node is allowed to enter, by checking if the node itself want to enter CS? if yes, did the requesting node come first? (checked by comparing Lamport clocks). If it answers no to the first question, then it will reply back immediately, letting the requesting node access to the CS (the requesting node will still have to wait for the rest of the peers to answer before entering), If yes to the first question and yes to the second, then it will also reply back, if otherwise it will defer the reply to when the node itself has been granted access to the CS. The following flowchart gives an overview of the process:

![Decision process for the nodes when receiving an request from the network](ricart-agrawala.png)

### R3 (Liveliness)

Every node in the system, will keep an internal queue of requests that when the nodes defer an reply it will add the request to the queue (if the request is sent after the node itself wants to enter the CS). When the node has been granted access to the CS, done the work that was necessary, it will then reply to all of the request in queue that they can access the CS.

## Discussion of Ricart-Argawala implementation

Node 1 broadcasts a CS request:

`2025/11/11 17:40:36 [Node 1] Broadcasting request for CS (Clock=3)`


Node 2 and Node 3 each receive this request and send a reply:

```log
2025/11/11 17:40:36 [Node 2] Received Request from 1 (Clock=3)
2025/11/11 17:40:36 [Node 2] Sending REPLY to 1
```

[Node 2] Received Request from 1 (Clock=3)
[Node 2] Sending REPLY to 1
[Node 3] Received Request from 1 (Clock=3)
[Node 3] Sending REPLY to 1


Once Node 1 collects both replies, it enters the CS:

[Node 1] Got reply from 3 (1/2)
[Node 1] Got reply from 2 (2/2)
[Node 1] ENTERING CRITICAL SECTION at clock 8


At the same moment, Node 2 also requests the CS:

[Node 2] Broadcasting request for CS (Clock=5)


But because Node 1 is already inside its CS and has an earlier timestamp, Node 2 must wait for Node 1 to release the CS.
Once Node 1 leaves:

[Node 1] LEAVING CRITICAL SECTION at clock 10


it sends any deferred replies, allowing Node 2 to enter:

[Node 2] ENTERING CRITICAL SECTION at clock 11


This sequence demonstrates correct mutual exclusion — only one node is in the CS at a time, and entry is granted in timestamp order.

## Appendix

### Log - Node 1

```text
2025/11/11 17:40:32 [Node 1] gRPC server now listening on :5001...
2025/11/11 17:40:32 [Node 1] Advertised on network (port 5001)
2025/11/11 17:40:34 [Node 1] Received Request from 3 (Clock=1)
2025/11/11 17:40:34 [Node 1] Sending REPLY to 3
2025/11/11 17:40:35 [Node 1] Waiting for peers to appear...
2025/11/11 17:40:35 [Node 1] Discovered new peer: node-3 (192.168.2.75:5003)
2025/11/11 17:40:35 [Node 1] Discovered new peer: node-2 (192.168.2.75:5002)
2025/11/11 17:40:35 [Node 1] Added new peer: 192.168.2.75:5003
2025/11/11 17:40:35 [Node 1] Added new peer: 192.168.2.75:5002
2025/11/11 17:40:36 [Node 1] Broadcasting request for CS (Clock=3)
2025/11/11 17:40:36 [Node 1] Got reply from 3 (1/2)
2025/11/11 17:40:36 [Node 1] Got reply from 2 (2/2)
2025/11/11 17:40:36 [Node 1] ENTERING CRITICAL SECTION at clock 8
2025/11/11 17:40:37 [Node 1] Received Request from 2 (Clock=5)
2025/11/11 17:40:37 [Node 1] Sending REPLY to 2
2025/11/11 17:40:39 [Node 1] LEAVING CRITICAL SECTION at clock 10
2025/11/11 17:40:44 [Node 1] Broadcasting request for CS (Clock=12)
2025/11/11 17:40:44 [Node 1] Got reply from 2 (1/2)
2025/11/11 17:40:44 [Node 1] Got reply from 3 (2/2)
2025/11/11 17:40:44 [Node 1] Received Request from 3 (Clock=14)
2025/11/11 17:40:44 [Node 1] Sending REPLY to 3
2025/11/11 17:40:44 [Node 1] ENTERING CRITICAL SECTION at clock 17
2025/11/11 17:40:44 [Node 1] Received Request from 2 (Clock=16)
2025/11/11 17:40:44 [Node 1] Sending REPLY to 2
2025/11/11 17:40:47 [Node 1] LEAVING CRITICAL SECTION at clock 19
2025/11/11 17:40:51 [Node 1] Broadcasting request for CS (Clock=21)
2025/11/11 17:40:51 [Node 1] Got reply from 3 (1/2)
2025/11/11 17:40:51 [Node 1] Got reply from 2 (2/2)
2025/11/11 17:40:51 [Node 1] ENTERING CRITICAL SECTION at clock 27
2025/11/11 17:40:52 [Node 1] Received Request from 2 (Clock=27)
2025/11/11 17:40:52 [Node 1] Sending REPLY to 2
2025/11/11 17:40:53 [Node 1] Received Request from 3 (Clock=29)
2025/11/11 17:40:53 [Node 1] Sending REPLY to 3
2025/11/11 17:40:54 [Node 1] LEAVING CRITICAL SECTION at clock 30
2025/11/11 17:40:58 [Node 1] Received Request from 2 (Clock=35)
2025/11/11 17:40:58 [Node 1] Sending REPLY to 2
2025/11/11 17:40:59 [Node 1] Broadcasting request for CS (Clock=37)
2025/11/11 17:40:59 [Node 1] Got reply from 3 (1/2)
2025/11/11 17:40:59 [Node 1] Got reply from 2 (2/2)
2025/11/11 17:40:59 [Node 1] ENTERING CRITICAL SECTION at clock 42
2025/11/11 17:40:59 [Node 1] Received Request from 3 (Clock=39)
2025/11/11 17:40:59 [Node 1] Sending REPLY to 3
2025/11/11 17:41:02 [Node 1] LEAVING CRITICAL SECTION at clock 44
2025/11/11 17:41:05 [Node 1] Decided not to enter CS this time
2025/11/11 17:41:06 [Node 1] Received Request from 2 (Clock=44)
2025/11/11 17:41:06 [Node 1] Sending REPLY to 2
2025/11/11 17:41:06 [Node 1] Received Request from 3 (Clock=50)
2025/11/11 17:41:06 [Node 1] Sending REPLY to 3
2025/11/11 17:41:09 [Node 1] Decided not to enter CS this time
2025/11/11 17:41:10 [Node 1] Received Request from 2 (Clock=55)
2025/11/11 17:41:10 [Node 1] Sending REPLY to 2
2025/11/11 17:41:12 [Node 1] Received Request from 3 (Clock=58)
2025/11/11 17:41:12 [Node 1] Sending REPLY to 3
2025/11/11 17:41:12 [Node 1] Broadcasting request for CS (Clock=60)
2025/11/11 17:41:12 [Node 1] Got reply from 3 (1/2)
2025/11/11 17:41:12 [Node 1] Got reply from 2 (2/2)
2025/11/11 17:41:13 [Node 1] ENTERING CRITICAL SECTION at clock 64
2025/11/11 17:41:16 [Node 1] LEAVING CRITICAL SECTION at clock 65
2025/11/11 17:41:17 [Node 1] Decided not to enter CS this time
2025/11/11 17:41:17 [Node 1] Received Request from 2 (Clock=63)
2025/11/11 17:41:17 [Node 1] Sending REPLY to 2
2025/11/11 17:41:20 [Node 1] Received Request from 3 (Clock=66)
2025/11/11 17:41:20 [Node 1] Sending REPLY to 3
2025/11/11 17:41:20 [Node 1] Broadcasting request for CS (Clock=69)
2025/11/11 17:41:20 [Node 1] Got reply from 3 (1/2)
2025/11/11 17:41:20 [Node 1] Got reply from 2 (2/2)
2025/11/11 17:41:21 [Node 1] ENTERING CRITICAL SECTION at clock 75
2025/11/11 17:41:24 [Node 1] LEAVING CRITICAL SECTION at clock 76
2025/11/11 17:41:25 [Node 1] Received Request from 2 (Clock=74)
2025/11/11 17:41:25 [Node 1] Sending REPLY to 2
2025/11/11 17:41:25 [Node 1] Received Request from 3 (Clock=77)
2025/11/11 17:41:25 [Node 1] Sending REPLY to 3
2025/11/11 17:41:28 [Node 1] Broadcasting request for CS (Clock=80)
2025/11/11 17:41:28 [Node 1] Got reply from 2 (1/2)
2025/11/11 17:41:28 [Node 1] Got reply from 3 (2/2)
2025/11/11 17:41:29 [Node 1] ENTERING CRITICAL SECTION at clock 86
2025/11/11 17:41:31 [Node 1] Received Request from 3 (Clock=87)
2025/11/11 17:41:31 [Node 1] Sending REPLY to 3
2025/11/11 17:41:32 [Node 1] LEAVING CRITICAL SECTION at clock 88
2025/11/11 17:41:32 [Node 1] Received Request from 2 (Clock=89)
2025/11/11 17:41:32 [Node 1] Sending REPLY to 2
```

### Log - Node 2

```text
2025/11/11 17:40:31 [Node 2] gRPC server now listening on :5002...
2025/11/11 17:40:31 [Node 2] Advertised on network (port 5002)
2025/11/11 17:40:34 [Node 2] Received Request from 3 (Clock=1)
2025/11/11 17:40:34 [Node 2] Sending REPLY to 3
2025/11/11 17:40:34 [Node 2] Waiting for peers to appear...
2025/11/11 17:40:34 [Node 2] Discovered new peer: node-3 (192.168.2.75:5003)
2025/11/11 17:40:34 [Node 2] Discovered new peer: node-1 (192.168.2.75:5001)
2025/11/11 17:40:34 [Node 2] Added new peer: 192.168.2.75:5003
2025/11/11 17:40:34 [Node 2] Added new peer: 192.168.2.75:5001
2025/11/11 17:40:35 [Node 2] Decided not to enter CS this time
2025/11/11 17:40:36 [Node 2] Received Request from 1 (Clock=3)
2025/11/11 17:40:36 [Node 2] Sending REPLY to 1
2025/11/11 17:40:37 [Node 2] Broadcasting request for CS (Clock=5)
2025/11/11 17:40:37 [Node 2] Got reply from 3 (1/2)
2025/11/11 17:40:37 [Node 2] Got reply from 1 (2/2)
2025/11/11 17:40:37 [Node 2] ENTERING CRITICAL SECTION at clock 11
2025/11/11 17:40:40 [Node 2] LEAVING CRITICAL SECTION at clock 12
2025/11/11 17:40:44 [Node 2] Received Request from 1 (Clock=12)
2025/11/11 17:40:44 [Node 2] Sending REPLY to 1
2025/11/11 17:40:44 [Node 2] Received Request from 3 (Clock=14)
2025/11/11 17:40:44 [Node 2] Sending REPLY to 3
2025/11/11 17:40:44 [Node 2] Broadcasting request for CS (Clock=16)
2025/11/11 17:40:44 [Node 2] Got reply from 3 (1/2)
2025/11/11 17:40:44 [Node 2] Got reply from 1 (2/2)
2025/11/11 17:40:45 [Node 2] ENTERING CRITICAL SECTION at clock 23
2025/11/11 17:40:48 [Node 2] LEAVING CRITICAL SECTION at clock 24
2025/11/11 17:40:51 [Node 2] Received Request from 1 (Clock=21)
2025/11/11 17:40:51 [Node 2] Sending REPLY to 1
2025/11/11 17:40:52 [Node 2] Broadcasting request for CS (Clock=27)
2025/11/11 17:40:52 [Node 2] Got reply from 1 (1/2)
2025/11/11 17:40:52 [Node 2] Got reply from 3 (2/2)
2025/11/11 17:40:53 [Node 2] ENTERING CRITICAL SECTION at clock 31
2025/11/11 17:40:53 [Node 2] Received Request from 3 (Clock=29)
2025/11/11 17:40:53 [Node 2] Sending REPLY to 3
2025/11/11 17:40:56 [Node 2] LEAVING CRITICAL SECTION at clock 33
2025/11/11 17:40:58 [Node 2] Broadcasting request for CS (Clock=35)
2025/11/11 17:40:58 [Node 2] Got reply from 3 (1/2)
2025/11/11 17:40:58 [Node 2] Got reply from 1 (2/2)
2025/11/11 17:40:59 [Node 2] ENTERING CRITICAL SECTION at clock 39
2025/11/11 17:40:59 [Node 2] Received Request from 1 (Clock=37)
2025/11/11 17:40:59 [Node 2] Sending REPLY to 1
2025/11/11 17:40:59 [Node 2] Received Request from 3 (Clock=39)
2025/11/11 17:40:59 [Node 2] Sending REPLY to 3
2025/11/11 17:41:02 [Node 2] LEAVING CRITICAL SECTION at clock 42
2025/11/11 17:41:06 [Node 2] Broadcasting request for CS (Clock=44)
2025/11/11 17:41:06 [Node 2] Got reply from 3 (1/2)
2025/11/11 17:41:06 [Node 2] Got reply from 1 (2/2)
2025/11/11 17:41:06 [Node 2] Received Request from 3 (Clock=50)
2025/11/11 17:41:06 [Node 2] Sending REPLY to 3
2025/11/11 17:41:06 [Node 2] ENTERING CRITICAL SECTION at clock 52
2025/11/11 17:41:09 [Node 2] LEAVING CRITICAL SECTION at clock 53
2025/11/11 17:41:10 [Node 2] Broadcasting request for CS (Clock=55)
2025/11/11 17:41:10 [Node 2] Got reply from 1 (1/2)
2025/11/11 17:41:10 [Node 2] Got reply from 3 (2/2)
2025/11/11 17:41:11 [Node 2] ENTERING CRITICAL SECTION at clock 58
2025/11/11 17:41:12 [Node 2] Received Request from 3 (Clock=58)
2025/11/11 17:41:12 [Node 2] Sending REPLY to 3
2025/11/11 17:41:12 [Node 2] Received Request from 1 (Clock=60)
2025/11/11 17:41:12 [Node 2] Sending REPLY to 1
2025/11/11 17:41:14 [Node 2] LEAVING CRITICAL SECTION at clock 61
2025/11/11 17:41:17 [Node 2] Broadcasting request for CS (Clock=63)
2025/11/11 17:41:17 [Node 2] Got reply from 1 (1/2)
2025/11/11 17:41:17 [Node 2] Got reply from 3 (2/2)
2025/11/11 17:41:18 [Node 2] ENTERING CRITICAL SECTION at clock 69
2025/11/11 17:41:20 [Node 2] Received Request from 3 (Clock=66)
2025/11/11 17:41:20 [Node 2] Sending REPLY to 3
2025/11/11 17:41:20 [Node 2] Received Request from 1 (Clock=69)
2025/11/11 17:41:20 [Node 2] Sending REPLY to 1
2025/11/11 17:41:21 [Node 2] LEAVING CRITICAL SECTION at clock 72
2025/11/11 17:41:25 [Node 2] Broadcasting request for CS (Clock=74)
2025/11/11 17:41:25 [Node 2] Got reply from 1 (1/2)
2025/11/11 17:41:25 [Node 2] Got reply from 3 (2/2)
2025/11/11 17:41:25 [Node 2] Received Request from 3 (Clock=77)
2025/11/11 17:41:25 [Node 2] Sending REPLY to 3
2025/11/11 17:41:25 [Node 2] ENTERING CRITICAL SECTION at clock 81
2025/11/11 17:41:28 [Node 2] Received Request from 1 (Clock=80)
2025/11/11 17:41:28 [Node 2] Sending REPLY to 1
2025/11/11 17:41:28 [Node 2] LEAVING CRITICAL SECTION at clock 83
2025/11/11 17:41:31 [Node 2] Received Request from 3 (Clock=87)
2025/11/11 17:41:31 [Node 2] Sending REPLY to 3
2025/11/11 17:41:32 [Node 2] Broadcasting request for CS (Clock=89)
2025/11/11 17:41:32 [Node 2] Got reply from 1 (1/2)
2025/11/11 17:41:32 [Node 2] Got reply from 3 (2/2)
2025/11/11 17:41:32 [Node 2] ENTERING CRITICAL SECTION at clock 93

```

### Log - Node 3

```text
2025/11/11 17:40:30 [Node 3] gRPC server now listening on :5003...
2025/11/11 17:40:30 [Node 3] Advertised on network (port 5003)
2025/11/11 17:40:33 [Node 3] Waiting for peers to appear...
2025/11/11 17:40:33 [Node 3] Discovered new peer: node-2 (192.168.2.75:5002)
2025/11/11 17:40:33 [Node 3] Discovered new peer: node-1 (192.168.2.75:5001)
2025/11/11 17:40:33 [Node 3] Added new peer: 192.168.2.75:5002
2025/11/11 17:40:33 [Node 3] Added new peer: 192.168.2.75:5001
2025/11/11 17:40:34 [Node 3] Broadcasting request for CS (Clock=1)
2025/11/11 17:40:34 [Node 3] Got reply from 1 (2/2)
2025/11/11 17:40:34 [Node 3] Got reply from 2 (1/2)
2025/11/11 17:40:34 [Node 3] ENTERING CRITICAL SECTION at clock 4
2025/11/11 17:40:36 [Node 3] Received Request from 1 (Clock=3)
2025/11/11 17:40:36 [Node 3] Sending REPLY to 1
2025/11/11 17:40:37 [Node 3] Received Request from 2 (Clock=5)
2025/11/11 17:40:37 [Node 3] Sending REPLY to 2
2025/11/11 17:40:37 [Node 3] LEAVING CRITICAL SECTION at clock 7
2025/11/11 17:40:40 [Node 3] Decided not to enter CS this time
2025/11/11 17:40:44 [Node 3] Received Request from 1 (Clock=12)
2025/11/11 17:40:44 [Node 3] Sending REPLY to 1
2025/11/11 17:40:44 [Node 3] Broadcasting request for CS (Clock=14)
2025/11/11 17:40:44 [Node 3] Got reply from 1 (1/2)
2025/11/11 17:40:44 [Node 3] Got reply from 2 (2/2)
2025/11/11 17:40:44 [Node 3] ENTERING CRITICAL SECTION at clock 19
2025/11/11 17:40:44 [Node 3] Received Request from 2 (Clock=16)
2025/11/11 17:40:44 [Node 3] Sending REPLY to 2
2025/11/11 17:40:47 [Node 3] LEAVING CRITICAL SECTION at clock 21
2025/11/11 17:40:49 [Node 3] Decided not to enter CS this time
2025/11/11 17:40:51 [Node 3] Received Request from 1 (Clock=21)
2025/11/11 17:40:51 [Node 3] Sending REPLY to 1
2025/11/11 17:40:52 [Node 3] Received Request from 2 (Clock=27)
2025/11/11 17:40:52 [Node 3] Sending REPLY to 2
2025/11/11 17:40:53 [Node 3] Broadcasting request for CS (Clock=29)
2025/11/11 17:40:53 [Node 3] Got reply from 1 (1/2)
2025/11/11 17:40:53 [Node 3] Got reply from 2 (2/2)
2025/11/11 17:40:54 [Node 3] ENTERING CRITICAL SECTION at clock 34
2025/11/11 17:40:57 [Node 3] LEAVING CRITICAL SECTION at clock 35
2025/11/11 17:40:58 [Node 3] Received Request from 2 (Clock=35)
2025/11/11 17:40:58 [Node 3] Sending REPLY to 2
2025/11/11 17:40:59 [Node 3] Received Request from 1 (Clock=37)
2025/11/11 17:40:59 [Node 3] Sending REPLY to 1
2025/11/11 17:40:59 [Node 3] Broadcasting request for CS (Clock=39)
2025/11/11 17:40:59 [Node 3] Got reply from 1 (1/2)
2025/11/11 17:40:59 [Node 3] Got reply from 2 (2/2)
2025/11/11 17:41:00 [Node 3] ENTERING CRITICAL SECTION at clock 46
2025/11/11 17:41:03 [Node 3] LEAVING CRITICAL SECTION at clock 47
2025/11/11 17:41:06 [Node 3] Received Request from 2 (Clock=44)
2025/11/11 17:41:06 [Node 3] Sending REPLY to 2
2025/11/11 17:41:06 [Node 3] Broadcasting request for CS (Clock=50)
2025/11/11 17:41:06 [Node 3] Got reply from 2 (1/2)
2025/11/11 17:41:06 [Node 3] Got reply from 1 (2/2)
2025/11/11 17:41:06 [Node 3] ENTERING CRITICAL SECTION at clock 54
2025/11/11 17:41:09 [Node 3] LEAVING CRITICAL SECTION at clock 55
2025/11/11 17:41:10 [Node 3] Received Request from 2 (Clock=55)
2025/11/11 17:41:10 [Node 3] Sending REPLY to 2
2025/11/11 17:41:12 [Node 3] Broadcasting request for CS (Clock=58)
2025/11/11 17:41:12 [Node 3] Got reply from 1 (1/2)
2025/11/11 17:41:12 [Node 3] Got reply from 2 (2/2)
2025/11/11 17:41:12 [Node 3] Received Request from 1 (Clock=60)
2025/11/11 17:41:12 [Node 3] Sending REPLY to 1
2025/11/11 17:41:12 [Node 3] ENTERING CRITICAL SECTION at clock 62
2025/11/11 17:41:15 [Node 3] LEAVING CRITICAL SECTION at clock 63
2025/11/11 17:41:17 [Node 3] Received Request from 2 (Clock=63)
2025/11/11 17:41:17 [Node 3] Sending REPLY to 2
2025/11/11 17:41:20 [Node 3] Broadcasting request for CS (Clock=66)
2025/11/11 17:41:20 [Node 3] Got reply from 1 (1/2)
2025/11/11 17:41:20 [Node 3] Got reply from 2 (2/2)
2025/11/11 17:41:20 [Node 3] Received Request from 1 (Clock=69)
2025/11/11 17:41:20 [Node 3] Sending REPLY to 1
2025/11/11 17:41:20 [Node 3] ENTERING CRITICAL SECTION at clock 73
2025/11/11 17:41:23 [Node 3] LEAVING CRITICAL SECTION at clock 74
2025/11/11 17:41:25 [Node 3] Received Request from 2 (Clock=74)
2025/11/11 17:41:25 [Node 3] Sending REPLY to 2
2025/11/11 17:41:25 [Node 3] Broadcasting request for CS (Clock=77)
2025/11/11 17:41:25 [Node 3] Got reply from 1 (2/2)
2025/11/11 17:41:25 [Node 3] Got reply from 2 (1/2)
2025/11/11 17:41:26 [Node 3] ENTERING CRITICAL SECTION at clock 83
2025/11/11 17:41:28 [Node 3] Received Request from 1 (Clock=80)
2025/11/11 17:41:28 [Node 3] Sending REPLY to 1
2025/11/11 17:41:29 [Node 3] LEAVING CRITICAL SECTION at clock 85
2025/11/11 17:41:31 [Node 3] Broadcasting request for CS (Clock=87)
2025/11/11 17:41:31 [Node 3] Got reply from 2 (1/2)
2025/11/11 17:41:31 [Node 3] Got reply from 1 (2/2)
2025/11/11 17:41:31 [Node 3] ENTERING CRITICAL SECTION at clock 90
2025/11/11 17:41:32 [Node 3] Received Request from 2 (Clock=89)
2025/11/11 17:41:32 [Node 3] Sending REPLY to 2
2025/11/11 17:41:34 [Node 3] LEAVING CRITICAL SECTION at clock 92

```

## Implementation
- Languages / frameworks / libraries used
- Important modules and brief description
- Key algorithms or protocols implemented
- Known limitations / trade-offs

## Evaluation
- Experimental setup (hardware, network, datasets)
- Metrics to measure
- Test cases and scenarios

## Results
- Tables / charts (embed images or markdown tables)
- Numeric outcomes with short interpretation

## Discussion
- Analysis of results
- Sources of error, reproducibility concerns
- Alternatives considered

## Conclusion and Future Work
- Summary of achievements
- Short list of possible improvements and next steps

## How to run / Reproduce
Prerequisites:
- OS, runtime, packages

Build & run (example):
```bash
# install deps
make install

# run tests
make test

# start service
./run.sh
```
Configuration: describe config files / environment variables.


```

## References
- [Paper / spec / library] Title — Author — Year
- URLs and dataset sources

## Appendix
- Raw logs, extended tables, command outputs
- Submission checklist:
    - [ ] Report written
    - [ ] Code compiles
    - [ ] Tests included
    - [ ] Instructions verified
