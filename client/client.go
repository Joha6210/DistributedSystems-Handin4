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

