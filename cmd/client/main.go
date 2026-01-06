package main

import (
	"flag"
	"fmt"
	"net"
	"net/rpc"
	"os"
	"strings"
	"time"

	"github.com/ankithrao/distributed-kv/internal/common"
	"github.com/ankithrao/distributed-kv/internal/raftlike"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8001", "node addr")
	op := flag.String("op", "get", "get|put|del")
	key := flag.String("key", "", "key")
	val := flag.String("val", "", "value for put")
	flag.Parse()

	if strings.TrimSpace(*key) == "" {
		fmt.Println("missing -key")
		os.Exit(1)
	}

	c, err := dial(*addr, 250*time.Millisecond)
	if err != nil {
		fmt.Println("dial:", err)
		os.Exit(1)
	}
	defer c.Close()

	switch *op {
	case "get":
		var rep raftlike.KVGetReply
		err = c.Call("Node.KVGet", raftlike.KVGetArgs{Req: common.ClientGetRequest{Key: *key}}, &rep)
		fmt.Printf("err=%v OK=%v value=%q msg=%q\n", err, rep.Res.OK, rep.Res.Value, rep.Res.Error)

	case "put":
		var rep raftlike.KVWriteReply
		err = c.Call("Node.KVWrite", raftlike.KVWriteArgs{Req: common.ClientWriteRequest{Op: common.OpPut, Key: *key, Value: *val}}, &rep)
		fmt.Printf("err=%v OK=%v leader=%q msg=%q\n", err, rep.Res.OK, rep.Res.Leader, rep.Res.Error)

	case "del":
		var rep raftlike.KVWriteReply
		err = c.Call("Node.KVWrite", raftlike.KVWriteArgs{Req: common.ClientWriteRequest{Op: common.OpDel, Key: *key}}, &rep)
		fmt.Printf("err=%v OK=%v leader=%q msg=%q\n", err, rep.Res.OK, rep.Res.Leader, rep.Res.Error)

	default:
		fmt.Println("unknown -op")
	}
}

func dial(addr string, timeout time.Duration) (*rpc.Client, error) {
	d := net.Dialer{Timeout: timeout}
	conn, err := d.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}
	_ = conn.SetDeadline(time.Now().Add(2 * timeout))
	return rpc.NewClient(conn), nil
}
