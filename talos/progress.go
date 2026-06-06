package talos

import (
	"fmt"
	"strings"
	"sync"

	"github.com/rs/zerolog/log"
)

// NodeProgress tracks progress for a single node
type NodeProgress struct {
	Node    string
	Status  string
	Percent int
	Message string
}

// ProgressReporter tracks and reports upgrade progress across nodes
type ProgressReporter struct {
	mu           sync.Mutex
	nodeProgress map[string]*NodeProgress
}

// NewProgressReporter creates a new progress reporter
func NewProgressReporter() *ProgressReporter {
	return &ProgressReporter{
		nodeProgress: make(map[string]*NodeProgress),
	}
}

// Update updates progress for a specific node
func (r *ProgressReporter) Update(node, status string, percent int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.nodeProgress[node] = &NodeProgress{
		Node:    node,
		Status:  status,
		Percent: percent,
	}

	log.Debug().
		Str("node", node).
		Str("status", status).
		Int("percent", percent).
		Msg("Progress updated")
}

// UpdateWithMessage updates progress with a custom message
func (r *ProgressReporter) UpdateWithMessage(node, status string, percent int, message string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.nodeProgress[node] = &NodeProgress{
		Node:    node,
		Status:  status,
		Percent: percent,
		Message: message,
	}

	log.Debug().
		Str("node", node).
		Str("status", status).
		Int("percent", percent).
		Str("message", message).
		Msg("Progress updated")
}

// Format returns a formatted progress message for all nodes
func (r *ProgressReporter) Format() string {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.nodeProgress) == 0 {
		return "No upgrade progress yet"
	}

	var parts []string
	for _, progress := range r.nodeProgress {
		if progress.Message != "" {
			parts = append(parts, fmt.Sprintf("%s: %s (%s)", progress.Node, progress.Status, progress.Message))
		} else {
			parts = append(parts, fmt.Sprintf("%s: %s (%d%%)", progress.Node, progress.Status, progress.Percent))
		}
	}

	return strings.Join(parts, "\n")
}

// GetNodeProgress returns progress for a specific node
func (r *ProgressReporter) GetNodeProgress(node string) *NodeProgress {
	r.mu.Lock()
	defer r.mu.Unlock()

	p := r.nodeProgress[node]
	if p == nil {
		return nil
	}
	return &NodeProgress{
		Node:    p.Node,
		Status:  p.Status,
		Percent: p.Percent,
		Message: p.Message,
	}
}

// Clear clears all progress tracking
func (r *ProgressReporter) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.nodeProgress = make(map[string]*NodeProgress)
}
