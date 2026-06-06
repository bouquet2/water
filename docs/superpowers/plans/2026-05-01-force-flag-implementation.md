# Force Flag Feature Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `-force` flag to bypass specific safety checks during cluster upgrades

**Architecture:** Custom ForceMode type implementing flag.Value interface, integrated into upgrade.Manager to selectively skip version validation, node readiness checks, and prerequisite validations based on active modes.

**Tech Stack:** Go standard library flag package, zerolog for logging, existing upgrade/version package

---

## File Structure

**New files:**
- `upgrade/force.go` - ForceMode type with flag.Value implementation, mode parsing, mode checking methods
- `upgrade/force_test.go` - Unit tests for ForceMode parsing and behavior

**Modified files:**
- `main.go` - Add `-force` flag definition, pass ForceMode to Manager
- `upgrade/manager.go` - Add ForceMode field, integrate check bypass logic at validation points
- `README.md` - Document `-force` flag usage with examples

---

### Task 1: ForceMode Type - Basic Structure and Parsing

**Files:**
- Create: `upgrade/force.go`

- [ ] **Step 1: Define ForceMode constants and type**

```go
package upgrade

import (
	"flag"
	"fmt"
	"strings"
)

type ForceMode string

const (
	ForceModeVersion      ForceMode = "version"
	ForceModeAvailability ForceMode = "availability"
	ForceModeReadiness    ForceMode = "readiness"
	ForceModeAll          ForceMode = "all"
)

type ForceModes struct {
	modes map[ForceMode]bool
}

func NewForceModes() *ForceModes {
	return &ForceModes{
		modes: make(map[ForceMode]bool),
	}
}
```

- [ ] **Step 2: Implement flag.Value interface (Set and String methods)**

```go
func (fm *ForceModes) Set(value string) error {
	if value == "" {
		fm.modes[ForceModeVersion] = true
		return nil
	}

	modes := strings.Split(value, ",")
	for _, modeStr := range modes {
		modeStr = strings.TrimSpace(strings.ToLower(modeStr))
		if modeStr == "" {
			return fmt.Errorf("invalid force mode: empty mode in '%s'", value)
		}

		mode := ForceMode(modeStr)
		switch mode {
		case ForceModeVersion, ForceModeAvailability, ForceModeReadiness, ForceModeAll:
			fm.modes[mode] = true
		default:
			return fmt.Errorf("invalid force mode '%s', available modes: version, availability, readiness, all", modeStr)
		}
	}

	if fm.modes[ForceModeAll] {
		fm.modes[ForceModeVersion] = true
		fm.modes[ForceModeAvailability] = true
		fm.modes[ForceModeReadiness] = true
	}

	return nil
}

func (fm *ForceModes) String() string {
	if len(fm.modes) == 0 {
		return ""
	}

	var active []string
	if fm.modes[ForceModeAll] {
		return "all"
	}

	for mode := range fm.modes {
		if mode != ForceModeAll {
			active = append(active, string(mode))
		}
	}

	return strings.Join(active, ",")
}
```

- [ ] **Step 3: Implement mode checking methods**

```go
func (fm *ForceModes) HasMode(mode ForceMode) bool {
	return fm.modes[mode]
}

func (fm *ForceModes) IsForcingVersion() bool {
	return fm.HasMode(ForceModeVersion)
}

func (fm *ForceModes) IsForcingAvailability() bool {
	return fm.HasMode(ForceModeAvailability)
}

func (fm *ForceModes) IsForcingReadiness() bool {
	return fm.HasMode(ForceModeReadiness)
}

func (fm *ForceModes) IsForcingAll() bool {
	return fm.HasMode(ForceModeAll)
}

func (fm *ForceModes) IsActive() bool {
	return len(fm.modes) > 0
}

func (fm *ForceModes) ModesString() string {
	return fm.String()
}
```

- [ ] **Step 4: Verify compilation**

Run: `go build`
Expected: Compilation succeeds with no errors

---

### Task 2: ForceMode Unit Tests

**Files:**
- Create: `upgrade/force_test.go`

- [ ] **Step 1: Write tests for valid single modes**

```go
package upgrade

import (
	"testing"
)

func TestForceModes_Set_ValidSingleModes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected ForceModes
	}{
		{
			name:  "version mode",
			input: "version",
			expected: ForceModes{modes: map[ForceMode]bool{
				ForceModeVersion: true,
			}},
		},
		{
			name:  "availability mode",
			input: "availability",
			expected: ForceModes{modes: map[ForceMode]bool{
				ForceModeAvailability: true,
			}},
		},
		{
			name:  "readiness mode",
			input: "readiness",
			expected: ForceModes{modes: map[ForceMode]bool{
				ForceModeReadiness: true,
			}},
		},
		{
			name:  "all mode",
			input: "all",
			expected: ForceModes{modes: map[ForceMode]bool{
				ForceModeAll:          true,
				ForceModeVersion:      true,
				ForceModeAvailability: true,
				ForceModeReadiness:    true,
			}},
		},
		{
			name:  "empty defaults to version",
			input: "",
			expected: ForceModes{modes: map[ForceMode]bool{
				ForceModeVersion: true,
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fm := NewForceModes()
			err := fm.Set(tt.input)
			if err != nil {
				t.Errorf("Set(%s) returned unexpected error: %v", tt.input, err)
			}

			for mode, expected := range tt.expected.modes {
				if fm.HasMode(mode) != expected {
					t.Errorf("HasMode(%s) = %v, expected %v", mode, fm.HasMode(mode), expected)
				}
			}
		})
	}
}
```

- [ ] **Step 2: Run tests to verify parsing works**

Run: `go test ./upgrade -v -run TestForceModes_Set_ValidSingleModes`
Expected: All tests PASS

- [ ] **Step 3: Write tests for valid combinations**

```go
func TestForceModes_Set_ValidCombinations(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected ForceModes
	}{
		{
			name:  "version and readiness",
			input: "version,readiness",
			expected: ForceModes{modes: map[ForceMode]bool{
				ForceModeVersion:   true,
				ForceModeReadiness: true,
			}},
		},
		{
			name:  "availability and readiness",
			input: "availability,readiness",
			expected: ForceModes{modes: map[ForceMode]bool{
				ForceModeAvailability: true,
				ForceModeReadiness:    true,
			}},
		},
		{
			name:  "version and availability",
			input: "version,availability",
			expected: ForceModes{modes: map[ForceMode]bool{
				ForceModeVersion:      true,
				ForceModeAvailability: true,
			}},
		},
		{
			name:  "all three modes",
			input: "version,availability,readiness",
			expected: ForceModes{modes: map[ForceMode]bool{
				ForceModeVersion:      true,
				ForceModeAvailability: true,
				ForceModeReadiness:    true,
			}},
		},
		{
			name:  "duplicate modes are deduplicated",
			input: "version,version",
			expected: ForceModes{modes: map[ForceMode]bool{
				ForceModeVersion: true,
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fm := NewForceModes()
			err := fm.Set(tt.input)
			if err != nil {
				t.Errorf("Set(%s) returned unexpected error: %v", tt.input, err)
			}

			for mode, expected := range tt.expected.modes {
				if fm.HasMode(mode) != expected {
					t.Errorf("HasMode(%s) = %v, expected %v", mode, fm.HasMode(mode), expected)
				}
			}
		})
	}
}
```

- [ ] **Step 4: Run tests to verify combinations**

Run: `go test ./upgrade -v -run TestForceModes_Set_ValidCombinations`
Expected: All tests PASS

- [ ] **Step 5: Write tests for invalid modes**

```go
func TestForceModes_Set_InvalidModes(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectedErr string
	}{
		{
			name:        "invalid mode name",
			input:       "invalid",
			expectedErr: "invalid force mode 'invalid'",
		},
		{
			name:        "invalid mode in combination",
			input:       "version,foo",
			expectedErr: "invalid force mode 'foo'",
		},
		{
			name:        "empty mode in list",
			input:       ",version",
			expectedErr: "invalid force mode: empty mode",
		},
		{
			name:        "trailing comma",
			input:       "version,",
			expectedErr: "invalid force mode: empty mode",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fm := NewForceModes()
			err := fm.Set(tt.input)
			if err == nil {
				t.Errorf("Set(%s) expected error, got nil", tt.input)
			}
			if !strings.Contains(err.Error(), tt.expectedErr) {
				t.Errorf("Set(%s) error = '%v', expected to contain '%s'", tt.input, err, tt.expectedErr)
			}
		})
	}
}
```

- [ ] **Step 6: Run tests to verify error handling**

Run: `go test ./upgrade -v -run TestForceModes_Set_InvalidModes`
Expected: All tests PASS

- [ ] **Step 7: Write tests for String() method**

```go
func TestForceModes_String(t *testing.T) {
	tests := []struct {
		name     string
		setup    string
		expected string
	}{
		{
			name:     "empty modes",
			setup:    "",
			expected: "",
		},
		{
			name:     "single mode version",
			setup:    "version",
			expected: "version",
		},
		{
			name:     "all mode",
			setup:    "all",
			expected: "all",
		},
		{
			name:     "combination",
			setup:    "version,readiness",
			expected: "version,readiness",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fm := NewForceModes()
			if tt.setup != "" || tt.name == "empty modes" {
				if tt.name != "empty modes" {
					_ = fm.Set(tt.setup)
				}
				result := fm.String()
				if result != tt.expected {
					t.Errorf("String() = '%s', expected '%s'", result, tt.expected)
				}
			}
		})
	}
}
```

- [ ] **Step 8: Run all ForceMode tests**

Run: `go test ./upgrade -v`
Expected: All tests PASS

---

### Task 3: Manager Integration - Add ForceMode Field

**Files:**
- Modify: `upgrade/manager.go:17-29`

- [ ] **Step 1: Add ForceModes field to Manager struct**

Locate the Manager struct definition at lines 17-20:

```go
type Manager struct {
	talosClient *talos.Client
	config      *config.Config
}
```

Change to:

```go
type Manager struct {
	talosClient *talos.Client
	config      *config.Config
	forceModes  *ForceModes
}
```

- [ ] **Step 2: Update NewManager constructor**

Locate NewManager at lines 22-28:

```go
func NewManager(talosClient *talos.Client, cfg *config.Config) *Manager {
	return &Manager{
		talosClient: talosClient,
		config:      cfg,
	}
}
```

Change to:

```go
func NewManager(talosClient *talos.Client, cfg *config.Config, forceModes *ForceModes) *Manager {
	if forceModes == nil {
		forceModes = NewForceModes()
	}
	return &Manager{
		talosClient: talosClient,
		config:      cfg,
		forceModes:  forceModes,
	}
}
```

- [ ] **Step 3: Add logging for active force modes in PerformUpgrade**

In PerformUpgrade() at line 82-83 after "Starting upgrade process" log, add:

```go
log.Info().Msg("Starting upgrade process")
startTime := time.Now()

if m.forceModes.IsActive() {
	log.Info().
		Str("modes", m.forceModes.ModesString()).
		Msg("Force modes active - bypassing safety checks")
}
```

- [ ] **Step 4: Verify compilation after Manager changes**

Run: `go build`
Expected: Compilation succeeds (will fail in main.go temporarily, fix in next task)

---

### Task 4: CLI Integration - Add Force Flag

**Files:**
- Modify: `main.go:24-35, 122`

- [ ] **Step 1: Add forceModes flag variable**

Locate flag variable declarations at lines 24-34:

```go
var (
	configPath        = flag.String("config", "", "Path to configuration file (default: search for water.yaml)")
	talosConfigPath   = flag.String("talosconfig", "", "Path to Talos client configuration file (default: ~/.talos/config)")
	kubeconfigPath    = flag.String("kubeconfig", "", "Path to kubeconfig file (default: ~/.kube/config or KUBECONFIG env var)")
	checkOnly         = flag.Bool("check-only", false, "Only check versions without performing upgrades")
	verbose           = flag.Bool("verbose", false, "Enable verbose logging")
	quiet             = flag.Bool("quiet", false, "Enable quiet mode (errors only)")
	version           = flag.Bool("version", false, "Show version information")
	talosUpgradeOrder = flag.String("talos-upgrade-order", "", "Override Talos upgrade order: 'control-plane-first' or 'workers-first'")
	k8sUpgradeOrder   = flag.String("k8s-upgrade-order", "", "Override Kubernetes upgrade order: 'control-plane-first' or 'workers-first'")
)
```

Add after k8sUpgradeOrder:

```go
var (
	configPath        = flag.String("config", "", "Path to configuration file (default: search for water.yaml)")
	talosConfigPath   = flag.String("talosconfig", "", "Path to Talos client configuration file (default: ~/.talos/config)")
	kubeconfigPath    = flag.String("kubeconfig", "", "Path to kubeconfig file (default: ~/.kube/config or KUBECONFIG env var)")
	checkOnly         = flag.Bool("check-only", false, "Only check versions without performing upgrades")
	verbose           = flag.Bool("verbose", false, "Enable verbose logging")
	quiet             = flag.Bool("quiet", false, "Enable quiet mode (errors only)")
	version           = flag.Bool("version", false, "Show version information")
	talosUpgradeOrder = flag.String("talos-upgrade-order", "", "Override Talos upgrade order: 'control-plane-first' or 'workers-first'")
	k8sUpgradeOrder   = flag.String("k8s-upgrade-order", "", "Override Kubernetes upgrade order: 'control-plane-first' or 'workers-first'")
)
forceModes := upgrade.NewForceModes()
flag.Var(forceModes, "force", "Force upgrade bypassing safety checks: 'version' (default), 'availability', 'readiness', 'all', or combine with commas")
```

- [ ] **Step 2: Update run() function signature and NewManager call**

Locate line 47:

```go
os.Exit(run(*configPath, *talosConfigPath, *kubeconfigPath, *checkOnly, *talosUpgradeOrder, *k8sUpgradeOrder))
```

Change to:

```go
os.Exit(run(*configPath, *talosConfigPath, *kubeconfigPath, *checkOnly, *talosUpgradeOrder, *k8sUpgradeOrder, forceModes))
```

Update run() function signature at line 50:

```go
func run(configPath, talosConfigPath, kubeconfigPath string, checkOnly bool, talosUpgradeOrder, k8sUpgradeOrder string) int {
```

Change to:

```go
func run(configPath, talosConfigPath, kubeconfigPath string, checkOnly bool, talosUpgradeOrder, k8sUpgradeOrder string, forceModes *upgrade.ForceModes) int {
```

Update NewManager call at line 122:

```go
upgradeManager := upgrade.NewManager(talosClient, cfg)
```

Change to:

```go
upgradeManager := upgrade.NewManager(talosClient, cfg, forceModes)
```

- [ ] **Step 3: Verify compilation and build**

Run: `go build`
Expected: Compilation succeeds, binary created

- [ ] **Step 4: Test flag parsing with invalid mode**

Run: `./water -force=invalid`
Expected: Error message "invalid force mode 'invalid', available modes: version, availability, readiness, all"

- [ ] **Step 5: Test flag parsing with valid modes**

Run: `./water -force=version,readiness -version`
Expected: Shows version, no error

Run: `./water -force=all -version`
Expected: Shows version, no error

---

### Task 5: Manager Integration - Bypass Version Availability Checks

**Files:**
- Modify: `upgrade/manager.go:122-127, 139-143`

- [ ] **Step 1: Bypass Talos version availability check**

Locate lines 122-127 in PerformUpgrade():

```go
// Check if target Talos version is available
if err := version.ValidateTargetVersion(m.config.Talos.Version, version.TalosRelease); err != nil {
	log.Warn().
		Str("target_version", m.config.Talos.Version).
		Msg("Target Talos version is not yet released - skipping Talos upgrade")
	// Don't perform Talos upgrade, but continue to check Kubernetes
} else if talosNeedsUpgrade {
```

Change to:

```go
// Check if target Talos version is available
if m.forceModes.IsForcingAvailability() {
	log.Debug().Str("mode", "availability").Msg("Skipping Talos version availability check due to force mode")
} else if err := version.ValidateTargetVersion(m.config.Talos.Version, version.TalosRelease); err != nil {
	log.Warn().
		Str("target_version", m.config.Talos.Version).
		Msg("Target Talos version is not yet released - skipping Talos upgrade")
	// Don't perform Talos upgrade, but continue to check Kubernetes
} else if talosNeedsUpgrade {
```

- [ ] **Step 2: Bypass Kubernetes version availability check**

Locate lines 139-143 in PerformUpgrade():

```go
// Check if target Kubernetes version is available
if err := version.ValidateTargetVersion(m.config.K8s.Version, version.KubernetesRelease); err != nil {
	log.Warn().
		Str("target_version", m.config.K8s.Version).
		Msg("Target Kubernetes version is not yet released - skipping Kubernetes upgrade")
} else {
```

Change to:

```go
// Check if target Kubernetes version is available
if m.forceModes.IsForcingAvailability() {
	log.Debug().Str("mode", "availability").Msg("Skipping Kubernetes version availability check due to force mode")
} else if err := version.ValidateTargetVersion(m.config.K8s.Version, version.KubernetesRelease); err != nil {
	log.Warn().
		Str("target_version", m.config.K8s.Version).
		Msg("Target Kubernetes version is not yet released - skipping Kubernetes upgrade")
} else {
```

- [ ] **Step 3: Verify compilation**

Run: `go build`
Expected: Compilation succeeds

---

### Task 6: Manager Integration - Bypass Version Matching Checks

**Files:**
- Modify: `upgrade/manager.go:112-136, 144-184, 823-856`

- [ ] **Step 1: Modify checkTalosUpgradeNeeded to handle force mode**

Locate checkTalosUpgradeNeeded at lines 823-856:

```go
func (m *Manager) checkTalosUpgradeNeeded(clusterInfo *talos.ClusterInfo) (bool, []string) {
	var nodesToUpgrade []string

	for _, node := range clusterInfo.Nodes {
		needsUpgrade, err := version.NeedsUpgrade(node.TalosVersion, m.config.Talos.Version)
		if err != nil {
			log.Warn().
				Err(err).
				Str("node", node.Name).
				Str("current_version", node.TalosVersion).
				Str("target_version", m.config.Talos.Version).
				Msg("Failed to check if node needs upgrade, assuming it does")
			nodesToUpgrade = append(nodesToUpgrade, node.Name)
			continue
		}

		if needsUpgrade {
			log.Debug().
				Str("node", node.Name).
				Str("current_version", node.TalosVersion).
				Str("target_version", m.config.Talos.Version).
				Msg("Node needs Talos upgrade")
			nodesToUpgrade = append(nodesToUpgrade, node.Name)
		} else {
			log.Debug().
				Str("node", node.Name).
				Str("current_version", node.TalosVersion).
				Str("target_version", m.config.Talos.Version).
				Msg("Node already at target version")
		}
	}

	return len(nodesToUpgrade) > 0, nodesToUpgrade
}
```

Change to:

```go
func (m *Manager) checkTalosUpgradeNeeded(clusterInfo *talos.ClusterInfo) (bool, []string) {
	if m.forceModes.IsForcingVersion() {
		log.Debug().Str("mode", "version").Msg("Forcing Talos upgrade on all nodes due to force mode")
		var allNodes []string
		for _, node := range clusterInfo.Nodes {
			allNodes = append(allNodes, node.Name)
		}
		return len(allNodes) > 0, allNodes
	}

	var nodesToUpgrade []string

	for _, node := range clusterInfo.Nodes {
		needsUpgrade, err := version.NeedsUpgrade(node.TalosVersion, m.config.Talos.Version)
		if err != nil {
			log.Warn().
				Err(err).
				Str("node", node.Name).
				Str("current_version", node.TalosVersion).
				Str("target_version", m.config.Talos.Version).
				Msg("Failed to check if node needs upgrade, assuming it does")
			nodesToUpgrade = append(nodesToUpgrade, node.Name)
			continue
		}

		if needsUpgrade {
			log.Debug().
				Str("node", node.Name).
				Str("current_version", node.TalosVersion).
				Str("target_version", m.config.Talos.Version).
				Msg("Node needs Talos upgrade")
			nodesToUpgrade = append(nodesToUpgrade, node.Name)
		} else {
			log.Debug().
				Str("node", node.Name).
				Str("current_version", node.TalosVersion).
				Str("target_version", m.config.Talos.Version).
				Msg("Node already at target version")
		}
	}

	return len(nodesToUpgrade) > 0, nodesToUpgrade
}
```

- [ ] **Step 2: Modify Kubernetes upgrade check to handle force mode**

Locate lines 144-184 in PerformUpgrade():

```go
} else {
	// Check if Kubernetes upgrade is needed. Consider both API server and kubelet versions.
	k8sNeedsUpgradeAPIServer, err := version.NeedsUpgrade(clusterInfo.K8sVersion, m.config.K8s.Version)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Errorf("failed to check Kubernetes API server version: %w", err))
	}

	// Inspect kubelet versions across nodes via Kubernetes API
	kubeletNeedsUpgrade := false
	if ctx := context.Background(); err == nil { // only attempt if no prior error
		if kubeClusterInfo, kErr := k8s.GetClusterInfo(ctx); kErr == nil {
			for _, n := range kubeClusterInfo.Nodes {
				if needs, vErr := version.NeedsUpgrade(n.KubeletVersion, m.config.K8s.Version); vErr == nil && needs {
					kubeletNeedsUpgrade = true
					break
				}
			}
		} else {
			log.Debug().Err(kErr).Msg("Failed to get Kubernetes node info for kubelet version check")
		}
	}

	k8sNeedsUpgrade := k8sNeedsUpgradeAPIServer || kubeletNeedsUpgrade

	if k8sNeedsUpgrade {
```

Change to:

```go
} else {
	// Check if Kubernetes upgrade is needed
	var k8sNeedsUpgrade bool
	if m.forceModes.IsForcingVersion() {
		log.Debug().Str("mode", "version").Msg("Forcing Kubernetes upgrade due to force mode")
		k8sNeedsUpgrade = true
	} else {
		// Check if Kubernetes upgrade is needed. Consider both API server and kubelet versions.
		k8sNeedsUpgradeAPIServer, err := version.NeedsUpgrade(clusterInfo.K8sVersion, m.config.K8s.Version)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("failed to check Kubernetes API server version: %w", err))
		}

		// Inspect kubelet versions across nodes via Kubernetes API
		kubeletNeedsUpgrade := false
		if ctx := context.Background(); err == nil { // only attempt if no prior error
			if kubeClusterInfo, kErr := k8s.GetClusterInfo(ctx); kErr == nil {
				for _, n := range kubeClusterInfo.Nodes {
					if needs, vErr := version.NeedsUpgrade(n.KubeletVersion, m.config.K8s.Version); vErr == nil && needs {
						kubeletNeedsUpgrade = true
						break
					}
				}
			} else {
				log.Debug().Err(kErr).Msg("Failed to get Kubernetes node info for kubelet version check")
			}
		}

		k8sNeedsUpgrade = k8sNeedsUpgradeAPIServer || kubeletNeedsUpgrade
	}

	if k8sNeedsUpgrade {
```

- [ ] **Step 3: Verify compilation**

Run: `go build`
Expected: Compilation succeeds

---

### Task 7: Manager Integration - Bypass Node Readiness Checks

**Files:**
- Modify: `upgrade/manager.go:703-743`

- [ ] **Step 1: Modify validateUpgradePrerequisites to handle readiness force mode**

Locate validateUpgradePrerequisites at lines 703-743:

```go
func (m *Manager) validateUpgradePrerequisites() error {
	log.Info().Msg("Validating upgrade prerequisites")

	// Get current cluster information
	clusterInfo, err := m.talosClient.GetClusterInfo()
	if err != nil {
		return fmt.Errorf("failed to get cluster information for validation: %w", err)
	}

	// Check if all nodes are ready
	var notReadyNodes []string
	for _, node := range clusterInfo.Nodes {
		if !node.Ready {
			notReadyNodes = append(notReadyNodes, node.Name)
		}
	}

	if len(notReadyNodes) > 0 {
		return fmt.Errorf("some nodes are not ready for upgrade: %v", notReadyNodes)
	}

	// Check if we have at least one control plane node
	var controlPlaneCount int
	for _, node := range clusterInfo.Nodes {
		if node.IsControlPlane {
			controlPlaneCount++
		}
	}

	if controlPlaneCount == 0 {
		return fmt.Errorf("no control plane nodes found in cluster")
	}

	log.Info().
		Int("total_nodes", len(clusterInfo.Nodes)).
		Int("control_plane_nodes", controlPlaneCount).
		Int("worker_nodes", len(clusterInfo.Nodes)-controlPlaneCount).
		Msg("Upgrade prerequisites validated successfully")

	return nil
}
```

Change to:

```go
func (m *Manager) validateUpgradePrerequisites() error {
	log.Info().Msg("Validating upgrade prerequisites")

	// Get current cluster information
	clusterInfo, err := m.talosClient.GetClusterInfo()
	if err != nil {
		if m.forceModes.IsForcingAll() {
			log.Debug().Str("mode", "all").Msg("Skipping cluster info check due to force mode")
			return nil
		}
		return fmt.Errorf("failed to get cluster information for validation: %w", err)
	}

	// Check if all nodes are ready (skip if forcing readiness)
	if m.forceModes.IsForcingReadiness() {
		log.Debug().Str("mode", "readiness").Msg("Skipping node readiness check due to force mode")
	} else {
		var notReadyNodes []string
		for _, node := range clusterInfo.Nodes {
			if !node.Ready {
				notReadyNodes = append(notReadyNodes, node.Name)
			}
		}

		if len(notReadyNodes) > 0 {
			return fmt.Errorf("some nodes are not ready for upgrade: %v", notReadyNodes)
		}
	}

	// Check if we have at least one control plane node (skip if forcing all)
	if m.forceModes.IsForcingAll() {
		log.Debug().Str("mode", "all").Msg("Skipping control plane check due to force mode")
	} else {
		var controlPlaneCount int
		for _, node := range clusterInfo.Nodes {
			if node.IsControlPlane {
				controlPlaneCount++
			}
		}

		if controlPlaneCount == 0 {
			return fmt.Errorf("no control plane nodes found in cluster")
		}
	}

	log.Info().
		Int("total_nodes", len(clusterInfo.Nodes)).
		Msg("Upgrade prerequisites validated (with force modes)")

	return nil
}
```

- [ ] **Step 2: Verify compilation**

Run: `go build`
Expected: Compilation succeeds

---

### Task 8: Manager Integration - Bypass Checks in CheckOnly Mode

**Files:**
- Modify: `upgrade/manager.go:753-767, 770-784`

- [ ] **Step 1: Add force mode handling in CheckOnly for Talos**

Locate CheckOnly at lines 753-767:

```go
// Check Talos version
talosNeedsUpgrade, err := version.NeedsUpgrade(clusterInfo.TalosVersion, m.config.Talos.Version)
if err != nil {
	return fmt.Errorf("failed to check Talos version: %w", err)
}

// Check if target Talos version is available
talosVersionAvailable := true
if err := version.ValidateTargetVersion(m.config.Talos.Version, version.TalosRelease); err != nil {
	log.Warn().
		Str("target_version", m.config.Talos.Version).
		Msg("Target Talos version is not yet released - skipping Talos upgrade check")
	talosVersionAvailable = false
	talosNeedsUpgrade = false // Don't upgrade if version not available
}
```

Change to:

```go
// Check Talos version
var talosNeedsUpgrade bool
if m.forceModes.IsForcingVersion() {
	log.Debug().Str("mode", "version").Msg("Forcing Talos upgrade check due to force mode")
	talosNeedsUpgrade = true
} else {
	talosNeedsUpgrade, err = version.NeedsUpgrade(clusterInfo.TalosVersion, m.config.Talos.Version)
	if err != nil {
		return fmt.Errorf("failed to check Talos version: %w", err)
	}
}

// Check if target Talos version is available
talosVersionAvailable := true
if m.forceModes.IsForcingAvailability() {
	log.Debug().Str("mode", "availability").Msg("Skipping Talos version availability check due to force mode")
} else if err := version.ValidateTargetVersion(m.config.Talos.Version, version.TalosRelease); err != nil {
	log.Warn().
		Str("target_version", m.config.Talos.Version).
		Msg("Target Talos version is not yet released - skipping Talos upgrade check")
	talosVersionAvailable = false
	talosNeedsUpgrade = false // Don't upgrade if version not available
}
```

- [ ] **Step 2: Add force mode handling in CheckOnly for Kubernetes**

Locate lines 770-784:

```go
// Check if target Kubernetes version is available
k8sNeedsUpgrade := false
k8sVersionAvailable := true
if err := version.ValidateTargetVersion(m.config.K8s.Version, version.KubernetesRelease); err != nil {
	log.Warn().
		Str("target_version", m.config.K8s.Version).
		Msg("Target Kubernetes version is not yet released - skipping Kubernetes upgrade check")
	k8sVersionAvailable = false
} else {
	// Check Kubernetes version only if target version is available
	var err error
	k8sNeedsUpgrade, err = version.NeedsUpgrade(clusterInfo.K8sVersion, m.config.K8s.Version)
	if err != nil {
		return fmt.Errorf("failed to check Kubernetes version: %w", err)
	}
}
```

Change to:

```go
// Check if target Kubernetes version is available
k8sNeedsUpgrade := false
k8sVersionAvailable := true
if m.forceModes.IsForcingAvailability() {
	log.Debug().Str("mode", "availability").Msg("Skipping Kubernetes version availability check due to force mode")
} else if err := version.ValidateTargetVersion(m.config.K8s.Version, version.KubernetesRelease); err != nil {
	log.Warn().
		Str("target_version", m.config.K8s.Version).
		Msg("Target Kubernetes version is not yet released - skipping Kubernetes upgrade check")
	k8sVersionAvailable = false
}

// Check Kubernetes version if not forcing availability check or if version is available
if m.forceModes.IsForcingVersion() {
	log.Debug().Str("mode", "version").Msg("Forcing Kubernetes upgrade check due to force mode")
	k8sNeedsUpgrade = true
} else if k8sVersionAvailable || m.forceModes.IsForcingAvailability() {
	var err error
	k8sNeedsUpgrade, err = version.NeedsUpgrade(clusterInfo.K8sVersion, m.config.K8s.Version)
	if err != nil {
		return fmt.Errorf("failed to check Kubernetes version: %w", err)
	}
}
```

- [ ] **Step 3: Verify compilation**

Run: `go build`
Expected: Compilation succeeds

---

### Task 9: Update README Documentation

**Files:**
- Modify: `README.md:8-19, 39-53`

- [ ] **Step 1: Add force flag to features list**

Locate features section at lines 8-19:

```markdown
## Features
* Minimalistic
* Declarative configuration through YAML
* Safe upgrades
  * Upgrades control-plane first, workers last
    * Adjustable through configuration
* Support for Talos and Kubernetes versions
* Version checking
  * Checks repositories of Kubernetes and Talos to make sure you're not trying to upgrade to a version that doesn't exist yet
  * Only performs upgrades when current versions don't match target versions
* Dry run mode
```

Change to:

```markdown
## Features
* Minimalistic
* Declarative configuration through YAML
* Safe upgrades
  * Upgrades control-plane first, workers last
    * Adjustable through configuration
* Support for Talos and Kubernetes versions
* Version checking
  * Checks repositories of Kubernetes and Talos to make sure you're not trying to upgrade to a version that doesn't exist yet
  * Only performs upgrades when current versions don't match target versions
* Dry run mode
* Force mode
  * Bypass safety checks with `-force` flag
  * Granular control over which checks to skip
```

- [ ] **Step 2: Add force flag usage section**

After the Upgrade Order Options section (after line 53), add:

```markdown
### Force Mode

The `-force` flag allows bypassing specific safety checks during upgrades. This is useful for edge cases or emergency scenarios.

**Usage:**
```bash
# Default: bypass version matching checks
water -force

# Bypass specific checks
water -force=availability    # Skip version availability checks
water -force=readiness       # Skip node readiness checks
water -force=all             # Bypass all safety checks

# Combine multiple modes
water -force=version,readiness
```

**Force Modes:**

- **`version`** (default): Skip version matching checks. Allows re-upgrading nodes that are already at the target version.
- **`availability`**: Skip version availability checks. Allows upgrading to unreleased or non-existent versions.
- **`readiness`**: Skip node readiness checks. Allows upgrading nodes that are not in Ready state.
- **`all`**: Bypass all safety checks (version matching, availability, readiness, and prerequisite validations).

**Warning:** Using force modes bypasses safety checks designed to prevent failed upgrades. Use with caution.
```

- [ ] **Step 3: Verify README formatting**

Run: `cat README.md`
Expected: README shows force flag documentation

---

### Task 10: Final Verification and Testing

**Files:**
- All files

- [ ] **Step 1: Run all tests**

Run: `go test ./... -v`
Expected: All tests PASS

- [ ] **Step 2: Verify compilation and build**

Run: `go build -o water_test`
Expected: Compilation succeeds, binary created

- [ ] **Step 3: Test force flag combinations manually**

Run: `./water_test -force -version`
Expected: Shows version, no error

Run: `./water_test -force=all -version`
Expected: Shows version, no error

Run: `./water_test -force=invalid -version`
Expected: Error message about invalid mode

- [ ] **Step 4: Cleanup test binary**

Run: `rm water_test`
Expected: Test binary removed

---

## Self-Review Checklist

After writing this plan, verify:

- [x] **Spec coverage:** All spec requirements have corresponding tasks
  - ForceMode type and parsing ✓ (Tasks 1-2)
  - CLI integration ✓ (Task 4)
  - Manager integration ✓ (Tasks 3, 5-8)
  - Documentation ✓ (Task 9)
  - Testing ✓ (Task 10)

- [x] **Placeholder scan:** No TBD, TODO, or incomplete steps

- [x] **Type consistency:** All ForceMode references use same type name and methods throughout

- [x] **No placeholders:** Each step has complete code or exact commands

---

## Execution Options

Plan complete and saved to `docs/superpowers/plans/2026-05-01-force-flag-implementation.md`. Two execution options:

**1. Subagent-Driven (recommended)** - I dispatch a fresh subagent per task, review between tasks, fast iteration

**2. Inline Execution** - Execute tasks in this session using executing-plans, batch execution with checkpoints

Which approach?