# Distributed Key-Value Store (Go)

A fault-tolerant, leader-based distributed key-value store implemented in Go, inspired by Raft-style consensus.  
The system supports linearizable writes, crash recovery using persistent logs, and basic performance metrics.

---

## Features

- Leader-based consensus with randomized election timeouts
- Replicated log for linearizable writes
- RPC-based client–server Get/Put APIs
- Persistent storage using BoltDB for crash recovery
- Automatic leader re-election on failures
- Metrics instrumentation for:
  - Write latency
  - Read/write counts
  - Leader change events

---

## Architecture Overview

```
Client
  |
  | RPC (Get / Put)
  v
Leader Node
  |
  | AppendEntries (log replication)
  v
Follower Nodes
```

---

## Node Roles & Consensus

```
Leader:
- Accepts all client write requests
- Appends operations to a replicated log
- Replicates log entries to followers
- Commits entries after majority acknowledgment

Followers:
- Replicate log entries from leader
- Apply committed log entries to local state
- Participate in leader election

Consensus:
- Raft-inspired leader election
- Randomized election timeouts to avoid split votes
- Periodic heartbeats to maintain leadership
```

---

## Persistence & Fault Tolerance

```
- All write operations are appended to a persistent log (BoltDB)
- On restart, nodes replay the log to rebuild in-memory state
- Followers can crash and rejoin without data loss
- Leader crashes trigger re-election
```

---

## Metrics Instrumentation

```
Collected Metrics:
- Write latency (end-to-end, client request to commit)
- Total read requests
- Total write requests
- Leader election events

Metrics are periodically logged to stdout for observation.
```

---

## Project Structure

```
.
├── cmd/
│   ├── client/        # CLI client for Get / Put requests
│   │   └── main.go
│   └── node/          # Node entry point
│       └── main.go
├── internal/
│   ├── common/        # Shared types and log entry definitions
│   ├── kv/            # Key-value state machine
│   ├── metrics/       # Metrics collection and logging
│   ├── raftlike/      # Consensus, leader election, RPC handlers
│   └── storage/       # BoltDB-based persistent storage
├── data/              # Runtime data directory (gitignored)
├── Makefile
├── go.mod
├── go.sum
└── README.md
```

---

## Running the System

### Start Nodes (in separate terminals)

```
make run1
make run2
make run3
```

Each node runs on a different port and forms a cluster.

---

## Client Usage

### Put a value
```
go run ./cmd/client -addr 127.0.0.1:8001 -op put -key foo -val bar
```

### Get a value
```
go run ./cmd/client -addr 127.0.0.1:8001 -op get -key foo
```

If the node is not the leader, the client receives a redirect hint.

---

## Limitations (Intentional)

```
- No log compaction or snapshots
- No dynamic cluster membership
- No network partition handling beyond leader election
- No read linearizability optimization (leader-based reads)
```

---

## Learning Outcomes

```
- Distributed systems fundamentals
- Leader election and consensus
- Log replication and commit rules
- Fault tolerance via persistence
- RPC-based system design
- Metrics-driven validation
```

---

## Notes

This project is designed as a learning-oriented, interview-ready implementation of a distributed system rather than a production-ready database.