package raftlike

import "github.com/ankithrao/distributed-kv/internal/common"

type RequestVoteArgs struct {
	Term        uint64
	CandidateID string
	LastIndex   uint64
	LastTerm    uint64
}
type RequestVoteReply struct {
	Term        uint64
	VoteGranted bool
}

type AppendEntriesArgs struct {
	Term         uint64
	LeaderID     string
	PrevIndex    uint64
	PrevTerm     uint64
	Entries      []common.LogEntry
	LeaderCommit uint64
}
type AppendEntriesReply struct {
	Term    uint64
	Success bool
	// if false, follower can include a hint (optional)
}

type KVWriteArgs struct {
	Req common.ClientWriteRequest
}
type KVWriteReply struct {
	Res common.ClientWriteResponse
}

type KVGetArgs struct {
	Req common.ClientGetRequest
}
type KVGetReply struct {
	Res common.ClientGetResponse
}
