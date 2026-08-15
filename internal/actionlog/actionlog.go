// Package actionlog records a lightweight, structured audit trail of every
// action the dashboard performs on a worktree: which action, which worktree,
// when, and the result.
package actionlog

import (
	"encoding/json"
	"log"
	"os"
	"sync"
	"time"
)

// Entry is one recorded action.
type Entry struct {
	Time     time.Time `json:"time"`
	Action   string    `json:"action"`
	Project  string    `json:"project"`
	Worktree string    `json:"worktree"`
	Result   string    `json:"result"`
}

// Logger writes action entries as JSON lines.
type Logger struct {
	mu sync.Mutex
	l  *log.Logger
	f  *os.File
}

// New returns a Logger writing to path (a file, appended), or to stderr when
// path is empty or cannot be opened.
func New(path string) *Logger {
	l := &Logger{}
	if path != "" {
		if f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644); err == nil {
			l.f = f
			l.l = log.New(f, "", 0)
			return l
		}
	}
	l.l = log.New(os.Stderr, "", 0)
	return l
}

// Close flushes and releases the log file, if one was opened.
func (l *Logger) Close() error {
	if l.f != nil {
		return l.f.Close()
	}
	return nil
}

// Log records a single action. A non-nil err is captured in Result.
func (l *Logger) Log(action, project, worktree string, err error) {
	e := Entry{
		Time:     time.Now().UTC(),
		Action:   action,
		Project:  project,
		Worktree: worktree,
	}
	if err != nil {
		e.Result = "error: " + err.Error()
	} else {
		e.Result = "ok"
	}
	b, _ := json.Marshal(e)
	l.mu.Lock()
	defer l.mu.Unlock()
	l.l.Print(string(b))
}
