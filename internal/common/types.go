package common

type OpType string

const (
	OpPut OpType = "PUT"
	OpDel OpType = "DEL"
)

type LogEntry struct {
	Index uint64
	Term  uint64
	Op    OpType
	Key   string
	Value string
}

type ClientWriteRequest struct {
	Op    OpType
	Key   string
	Value string
}

type ClientWriteResponse struct {
	OK     bool
	Leader string
	Error  string
}

type ClientGetRequest struct {
	Key string
}

type ClientGetResponse struct {
	OK    bool
	Value string
	Error string
}
