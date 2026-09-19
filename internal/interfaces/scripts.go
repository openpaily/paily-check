package interfaces

// ScriptType identifies the purpose of a JS script.
type ScriptType string

const (
	STypeMedia ScriptType = "media"
	STypeIP    ScriptType = "ip"
)

// Script holds a single JS script to execute via the goja engine.
type Script struct {
	ID            string     `yaml:"-"`
	Type          ScriptType `yaml:"type"`
	Content       string     `yaml:"content"`
	TimeoutMillis uint64     `yaml:"timeout,omitempty"`
}

// ScriptResult is the outcome of executing one script against one node.
// true  = the node unlocks this service
// false = blocked, failed, or timed out
type ScriptResult struct {
	Unlocked    bool
	TimeElapsed int64 // milliseconds
}
