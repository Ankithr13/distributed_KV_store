package raftlike

import (
	"errors"
	"log"
	"math/rand"
	"net"
	"net/rpc"
	"sync"
	"time"

	"github.com/ankithrao/distributed-kv/internal/common"
	"github.com/ankithrao/distributed-kv/internal/kv"
	"github.com/ankithrao/distributed-kv/internal/metrics"
	"github.com/ankithrao/distributed-kv/internal/storage"
)

type Role int

const (
	Follower Role = iota
	Candidate
	Leader
)

type Node struct {
	mu sync.Mutex

	id    string
	addr  string
	peers []string

	role Role

	term       uint64
	votedFor   string
	leaderHint string

	commitIndex uint64
	lastApplied uint64

	logLastIndex uint64
	logLastTerm  uint64

	store *storage.BoltStore
	kv    *kv.KV

	// leader state
	nextIndex map[string]uint64

	// timers
	electReset time.Time
	stopCh     chan struct{}

	electTimeout time.Duration //added later (stabilization of leader election)
}

func NewNode(id, addr string, peers []string, store *storage.BoltStore) (*Node, error) {
	lastIdx, err := store.LastLogIndex()
	if err != nil {
		return nil, err
	}

	n := &Node{
		id:           id,
		addr:         addr,
		peers:        peers,
		role:         Follower,
		store:        store,
		kv:           kv.New(store),
		logLastIndex: lastIdx,
		nextIndex:    map[string]uint64{},
		stopCh:       make(chan struct{}),
	}
	// restore meta
	if t, ok, _ := store.GetMetaU64("term"); ok {
		n.term = t
	}
	if ci, ok, _ := store.GetMetaU64("commitIndex"); ok {
		n.commitIndex = ci
		n.lastApplied = ci
	}
	// replay to applied state (best-effort)
	for i := uint64(1); i <= n.lastApplied; i++ {
		e, ok, _ := store.ReadLog(i)
		if ok {
			_ = n.kv.Apply(e)
		}
	}

	//  STABILIZATION CHANGE: initialize election timer + timeout ONCE
	n.mu.Lock()
	n.resetElectionTimerLocked()
	n.mu.Unlock()

	return n, nil
}

func (n *Node) Start() error {
	srv := rpc.NewServer()
	if err := srv.RegisterName("Node", n); err != nil {
		return err
	}

	ln, err := net.Listen("tcp", n.addr)
	if err != nil {
		return err
	}

	go func() {
		for {
			conn, e := ln.Accept()
			if e != nil {
				select {
				case <-n.stopCh:
					return
				default:
					continue
				}
			}
			go srv.ServeConn(conn)
		}
	}()

	go n.electionLoop()
	go n.applyLoop()
	go n.heartbeatLoop()
	log.Printf("[%s] listening on %s peers=%v", n.id, n.addr, n.peers)
	//  METRICS: start periodic metrics logging
	metrics.StartLogger(5 * time.Second)
	return nil
}

func (n *Node) Stop() { close(n.stopCh) }

//func (n *Node) electionTimeout() time.Duration {
//	return time.Duration(250+rand.Intn(250)) * time.Millisecond
//
// }

func (n *Node) resetElectionTimerLocked() {
	n.electReset = time.Now()
	// More stable: 900–1500ms election timeout (common in practice for local demos)
	n.electTimeout = time.Duration(900+rand.Intn(600)) * time.Millisecond
}

func (n *Node) electionLoop() {
	t := time.NewTicker(50 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-n.stopCh:
			return
		case <-t.C:
			n.mu.Lock()
			role := n.role
			elapsed := time.Since(n.electReset)
			to := n.electTimeout //  STABILIZATION CHANGE: use stored timeout
			n.mu.Unlock()

			if role != Leader && elapsed > to {
				n.startElection()
			}
		}
	}
}

func (n *Node) startElection() {
	n.mu.Lock()
	n.role = Candidate
	n.term++
	_ = n.store.PutMetaU64("term", n.term)
	n.votedFor = n.id

	//  STABILIZATION CHANGE: reset timer when election starts
	n.resetElectionTimerLocked()

	term := n.term
	lastIndex := n.logLastIndex
	lastTerm := n.logLastTerm
	n.mu.Unlock()

	var votes int = 1
	var wg sync.WaitGroup
	var muVotes sync.Mutex

	for _, p := range n.peers {
		p := p
		wg.Add(1)
		go func() {
			defer wg.Done()
			args := RequestVoteArgs{
				Term:        term,
				CandidateID: n.id,
				LastIndex:   lastIndex,
				LastTerm:    lastTerm,
			}
			var rep RequestVoteReply
			if err := call(p, "Node.RequestVote", args, &rep, 150*time.Millisecond); err != nil {
				return
			}
			n.mu.Lock()
			if rep.Term > n.term {
				n.term = rep.Term
				_ = n.store.PutMetaU64("term", n.term)
				n.role = Follower
				n.votedFor = ""
				n.mu.Unlock()
				return
			}
			n.mu.Unlock()
			if rep.VoteGranted {
				muVotes.Lock()
				votes++
				muVotes.Unlock()
			}
		}()
	}
	wg.Wait()

	n.mu.Lock()
	defer n.mu.Unlock()
	if n.role != Candidate || n.term != term {
		return
	}
	// majority
	if votes > (len(n.peers)+1)/2 {
		n.role = Leader
		//  METRICS: leader elected
		metrics.RecordLeaderChange()
		n.leaderHint = n.addr

		//  STABILIZATION CHANGE: stabilize leadership after win
		n.resetElectionTimerLocked()

		n.nextIndex = map[string]uint64{}
		for _, p := range n.peers {
			n.nextIndex[p] = n.logLastIndex + 1
		}
		log.Printf("[%s] became LEADER term=%d votes=%d", n.id, n.term, votes)
	} else {
		n.role = Follower
	}
}

func (n *Node) heartbeatLoop() {
	t := time.NewTicker(120 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-n.stopCh:
			return
		case <-t.C:
			n.mu.Lock()
			if n.role != Leader {
				n.mu.Unlock()
				continue
			}
			term := n.term
			leader := n.addr
			commit := n.commitIndex
			n.mu.Unlock()

			for _, p := range n.peers {
				go func(peer string) {
					// send empty append as heartbeat
					args := AppendEntriesArgs{
						Term:         term,
						LeaderID:     leader,
						PrevIndex:    0,
						PrevTerm:     0,
						Entries:      nil,
						LeaderCommit: commit,
					}
					var rep AppendEntriesReply
					_ = call(peer, "Node.AppendEntries", args, &rep, 150*time.Millisecond)
					n.mu.Lock()
					if rep.Term > n.term {
						n.term = rep.Term
						_ = n.store.PutMetaU64("term", n.term)
						n.role = Follower
						n.votedFor = ""
					}
					n.mu.Unlock()
				}(p)
			}
		}
	}
}

func (n *Node) applyLoop() {
	t := time.NewTicker(30 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-n.stopCh:
			return
		case <-t.C:
			n.mu.Lock()
			for n.lastApplied < n.commitIndex {
				n.lastApplied++
				idx := n.lastApplied
				e, ok, _ := n.store.ReadLog(idx)
				if ok {
					_ = n.kv.Apply(e)
				}
			}
			_ = n.store.PutMetaU64("commitIndex", n.commitIndex)
			n.mu.Unlock()
		}
	}
}

/* ================= RPC handlers ================= */

func (n *Node) RequestVote(args RequestVoteArgs, rep *RequestVoteReply) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	rep.Term = n.term
	rep.VoteGranted = false

	if args.Term < n.term {
		return nil
	}
	if args.Term > n.term {
		n.term = args.Term
		_ = n.store.PutMetaU64("term", n.term)
		n.role = Follower
		n.votedFor = ""
	}

	// basic log freshness check (simplified)
	if n.votedFor == "" || n.votedFor == args.CandidateID {
		n.votedFor = args.CandidateID
		rep.VoteGranted = true

		//  STABILIZATION CHANGE: valid leader activity resets timeout
		n.resetElectionTimerLocked()
	}
	rep.Term = n.term
	return nil
}

func (n *Node) AppendEntries(args AppendEntriesArgs, rep *AppendEntriesReply) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	rep.Term = n.term
	rep.Success = false

	if args.Term < n.term {
		return nil
	}
	// accept leader
	if args.Term > n.term {
		n.term = args.Term
		_ = n.store.PutMetaU64("term", n.term)
		n.votedFor = ""
	}
	n.role = Follower
	n.leaderHint = args.LeaderID

	//  STABILIZATION CHANGE: heartbeat resets timeout
	n.resetElectionTimerLocked()

	// append entries (no conflict optimization in v1)
	for _, e := range args.Entries {
		if e.Index == 0 {
			continue
		}
		_ = n.store.AppendLog(e)
		if e.Index > n.logLastIndex {
			n.logLastIndex = e.Index
			n.logLastTerm = e.Term
		}
	}
	if args.LeaderCommit > n.commitIndex {
		if args.LeaderCommit > n.logLastIndex {
			n.commitIndex = n.logLastIndex
		} else {
			n.commitIndex = args.LeaderCommit
		}
	}

	rep.Term = n.term
	rep.Success = true
	return nil
}

func (n *Node) KVGet(args KVGetArgs, rep *KVGetReply) error {
	//  METRICS: read observed
	metrics.RecordRead()
	val, ok, err := n.kv.Get(args.Req.Key)
	if err != nil {
		rep.Res = common.ClientGetResponse{OK: false, Error: err.Error()}
		return nil
	}
	if !ok {
		rep.Res = common.ClientGetResponse{OK: false, Error: "not found"}
		return nil
	}
	rep.Res = common.ClientGetResponse{OK: true, Value: val}
	return nil
}

func (n *Node) KVWrite(args KVWriteArgs, rep *KVWriteReply) error {
	start := time.Now() // 🔧 METRICS: measure write latency
	n.mu.Lock()
	if n.role != Leader {
		leader := n.leaderHint
		n.mu.Unlock()
		rep.Res = common.ClientWriteResponse{OK: false, Leader: leader, Error: "not leader"}
		return nil
	}

	// create new log entry
	n.logLastIndex++
	e := common.LogEntry{
		Index: n.logLastIndex,
		Term:  n.term,
		Op:    args.Req.Op,
		Key:   args.Req.Key,
		Value: args.Req.Value,
	}
	_ = n.store.AppendLog(e)
	n.logLastTerm = e.Term
	term := n.term
	commitCandidate := e.Index
	n.mu.Unlock()

	// replicate to peers
	acks := 1
	var wg sync.WaitGroup
	var muAcks sync.Mutex

	for _, p := range n.peers {
		wg.Add(1)
		go func(peer string) {
			defer wg.Done()
			app := AppendEntriesArgs{
				Term:         term,
				LeaderID:     n.addr,
				Entries:      []common.LogEntry{e},
				LeaderCommit: 0,
			}
			var r AppendEntriesReply
			if err := call(peer, "Node.AppendEntries", app, &r, 250*time.Millisecond); err != nil {
				return
			}
			if r.Success {
				muAcks.Lock()
				acks++
				muAcks.Unlock()
			} else if r.Term > term {
				// leader stepped down
				n.mu.Lock()
				if r.Term > n.term {
					n.term = r.Term
					_ = n.store.PutMetaU64("term", n.term)
					n.role = Follower
					n.votedFor = ""
				}
				n.mu.Unlock()
			}
		}(p)
	}
	wg.Wait()

	// majority commit
	if acks <= (len(n.peers)+1)/2 {
		rep.Res = common.ClientWriteResponse{OK: false, Error: "failed to reach majority"}
		return nil
	}

	n.mu.Lock()
	if commitCandidate > n.commitIndex {
		n.commitIndex = commitCandidate
	}
	n.mu.Unlock()

	rep.Res = common.ClientWriteResponse{OK: true}
	//  METRICS: successful write
	metrics.RecordWrite(time.Since(start))
	return nil
}

/* ================= helpers ================= */

func call(addr, method string, args any, reply any, timeout time.Duration) error {
	d := net.Dialer{Timeout: timeout}
	conn, err := d.Dial("tcp", addr)
	if err != nil {
		return err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(timeout))

	c := rpc.NewClient(conn)
	defer c.Close()
	return c.Call(method, args, reply)
}

var ErrRedirect = errors.New("redirect")
