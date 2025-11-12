# How to run

To start the nodes run:

```bash
go run node.go <node-id> <port>
```

f.ex. to create a node with node id 1 on port 5001:

```bash
go run node.go 1 5001
```

If you want to run n number of nodes:


```bash
go run node.go 1 5001  //In terminal 1
go run node.go 2 5002  //In terminal 2
go run node.go 3 5003  //In terminal 3
.
.
.
go run node.go n 5000+n  //In terminal n
```

Each node continously listens for other nodes with [zeroconf](https://github.com/grandcat/zeroconf) mDNS discovery with the service type:

```golang
"_ricartagrawala._tcp"
```

## Project structure

```bash
/ (repo)
├─ grpc/        # grpc .proto definition
├─ logs/        # logs 
├─ node.go      # Main node that contains all functionality
├─ go.mod       # Go module
├─ go.sum       # Go module
└─ readme.md    # This readme.md
```

- Zeroconf installation
As this project depends on the package `zeroconf`, installation is required:
`go get -u github.com/grandcat/zeroconf`