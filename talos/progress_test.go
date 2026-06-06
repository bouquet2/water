package talos

import (
	"strings"
	"testing"
)

func TestNewProgressReporter(t *testing.T) {
	reporter := NewProgressReporter()
	if reporter == nil {
		t.Error("NewProgressReporter returned nil")
	}
}

func TestProgressReporterUpdate(t *testing.T) {
	reporter := NewProgressReporter()

	reporter.Update("node1", "Installing", 50)
	reporter.Update("node2", "Downloading", 25)

	// Should track multiple nodes
	p1 := reporter.GetNodeProgress("node1")
	p2 := reporter.GetNodeProgress("node2")
	if p1 == nil || p2 == nil {
		t.Error("Expected both nodes to be tracked")
	}
	if p1 != nil && p1.Percent != 50 {
		t.Errorf("Expected node1 percent 50, got %d", p1.Percent)
	}
	if p2 != nil && p2.Percent != 25 {
		t.Errorf("Expected node2 percent 25, got %d", p2.Percent)
	}
}

func TestProgressReporterFormat(t *testing.T) {
	reporter := NewProgressReporter()

	reporter.Update("node1", "Installing", 50)

	msg := reporter.Format()
	if msg == "" {
		t.Error("Format returned empty string")
	}

	// Should contain node name and status
	if !strings.Contains(msg, "node1") || !strings.Contains(msg, "Installing") {
		t.Errorf("Format missing expected content: %s", msg)
	}
}

func TestProgressReporterGetNodeProgress(t *testing.T) {
	reporter := NewProgressReporter()

	// Non-existent node
	p := reporter.GetNodeProgress("nonexistent")
	if p != nil {
		t.Error("Expected nil for non-existent node")
	}

	// Existing node
	reporter.Update("node1", "Installing", 50)
	p = reporter.GetNodeProgress("node1")
	if p == nil {
		t.Fatal("Expected progress for node1, got nil")
	}
	if p.Node != "node1" {
		t.Errorf("Expected node 'node1', got %s", p.Node)
	}
	if p.Status != "Installing" {
		t.Errorf("Expected status 'Installing', got %s", p.Status)
	}
	if p.Percent != 50 {
		t.Errorf("Expected percent 50, got %d", p.Percent)
	}

	// Verify returned value is a copy, not pointer to internal
	p.Percent = 99
	p2 := reporter.GetNodeProgress("node1")
	if p2.Percent == 99 {
		t.Error("GetNodeProgress returned pointer to internal value, not a copy")
	}
}

func TestProgressReporterUpdateWithMessage(t *testing.T) {
	reporter := NewProgressReporter()

	reporter.UpdateWithMessage("node1", "Installing", 50, "package foo")

	p := reporter.GetNodeProgress("node1")
	if p == nil {
		t.Fatal("Expected progress for node1, got nil")
	}
	if p.Message != "package foo" {
		t.Errorf("Expected message 'package foo', got %s", p.Message)
	}

	// Verify Format includes message
	msg := reporter.Format()
	if !strings.Contains(msg, "package foo") {
		t.Errorf("Format missing message: %s", msg)
	}
}

func TestProgressReporterClear(t *testing.T) {
	reporter := NewProgressReporter()

	reporter.Update("node1", "Installing", 50)
	reporter.Update("node2", "Downloading", 25)

	// Verify nodes tracked
	if p := reporter.GetNodeProgress("node1"); p == nil {
		t.Error("Expected node1 to be tracked")
	}

	// Clear
	reporter.Clear()

	// Verify all cleared
	if p := reporter.GetNodeProgress("node1"); p != nil {
		t.Error("Expected node1 to be cleared")
	}
	if p := reporter.GetNodeProgress("node2"); p != nil {
		t.Error("Expected node2 to be cleared")
	}

	// Verify Format returns empty message
	msg := reporter.Format()
	if msg != "No upgrade progress yet" {
		t.Errorf("Expected 'No upgrade progress yet', got %s", msg)
	}
}

func TestProgressReporterFormatEmpty(t *testing.T) {
	reporter := NewProgressReporter()

	msg := reporter.Format()
	if msg != "No upgrade progress yet" {
		t.Errorf("Expected 'No upgrade progress yet', got %s", msg)
	}
}
