package main

import (
	"flag"
	"log"
	"strings"

	"github.com/ankithrao/distributed-kv/internal/raftlike"
	"github.com/ankithrao/distributed-kv/internal/storage"
)

func main() {
	id := flag.String("id", "n1", "node id")
	addr := flag.String("addr", "127.0.0.1:8001", "listen addr")
	peers := flag.String("peers", "", "comma-separated peer addrs (no self)")
	data := flag.String("data", "./data/n1", "data dir")
	flag.Parse()

	var ps []string
	if strings.TrimSpace(*peers) != "" {
		for _, p := range strings.Split(*peers, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				ps = append(ps, p)
			}
		}
	}

	st, err := storage.Open(*data)
	if err != nil {
		log.Fatal(err)
	}
	defer st.Close()

	n, err := raftlike.NewNode(*id, *addr, ps, st)
	if err != nil {
		log.Fatal(err)
	}
	if err := n.Start(); err != nil {
		log.Fatal(err)
	}

	select {}
}
