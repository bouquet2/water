# Force Flag Feature Design

## Overview

Add a `-force` command-line flag to the water upgrade tool that allows bypassing specific safety checks during cluster upgrades. This provides operators with granular control over which validations to skip in edge cases or emergency scenarios.

## Command-Line Interface

### Flag Syntax

The `-force` flag supports multiple modes that can be specified individually or combined:

- `-force` (default: bypass version checks only)
- `-force=version` (explicit version mode)
- `-force=readiness` (skip node readiness checks)
- `-force=availability` (skip version availability checks only)
- `-force=all` (bypass all safety checks)
- `-force=version,readiness` (combine multiple modes)

### Force Modes

Each mode bypasses specific safety checks:

- `version` (default): Skip version matching checks. Allows re-upgrading nodes that are already at the target version. Does NOT skip availability checks.
- `availability`: Skip version availability checks only. Allows upgrading to unreleased or non-existent versions. Still checks version matching and node readiness.
- `readiness`: Skip node readiness checks. Allows upgrading nodes that are not in Ready state. Still checks versions.
- `all`: Bypass all safety checks - version matching, version availability, node readiness, and prerequisite validations. This mode implies all other modes.

### Mode Combinations

Modes can be combined using comma syntax:
- `-force=version,readiness` - Skip version matching AND node readiness checks
- `-force=availability,readiness` - Skip availability AND readiness checks
- `-force=all` is standalone and supersedes any other modes if combined

## Architecture

### Components

1. **ForceMode Type** (`upgrade/force.go`)
   - Custom flag type implementing `flag.Value` interface
   - Set of active force modes
   - Parsing logic for comma-separated values
   - Methods to check if specific modes are active

2. **CLI Integration** (`main.go`)
   - Add `-force` flag definition
   - Parse and validate force modes
   - Pass ForceMode to upgrade.Manager

3. **Manager Integration** (`upgrade/manager.go`)
   - Accept ForceMode parameter in constructor
   - Check active modes before each validation step:
     - `version.ValidateTargetVersion()` calls
     - `version.NeedsUpgrade()` calls
     - Node readiness checks in `validateUpgradePrerequisites()`
     - All prerequisite validations
   - Log which checks are bypassed

### Data Flow

1. CLI parses `-force` argument into ForceMode set
2. `main.go` validates ForceMode and passes to Manager constructor
3. Manager stores ForceMode and checks it before each validation:
   - Before version availability validation
   - Before version matching checks
   - Before node readiness checks
   - Before prerequisite validation
4. Manager logs active force modes at startup

## Error Handling & User Experience

### Logging

- At startup: Log active force modes at Info level
  ```
  log.Info().Str("modes", "version,readiness").Msg("Force modes active - bypassing safety checks")
  ```
- For each bypassed check: Log at Debug level that check is being skipped
  ```
  log.Debug().Str("mode", "readiness").Msg("Skipping node readiness check due to force mode")
  ```

### Error Messages

- Invalid mode names fail immediately with helpful error:
  ```
  Error: invalid force mode 'invalid', available modes: version, availability, readiness, all
  ```
- Parsing errors show usage example
- Empty `-force` defaults to `version` mode (no error)

### Behavior Rules

- `all` mode implies all other modes - no need to combine
- If `all` is combined with other modes, `all` supersedes
- Duplicate modes are deduplicated silently
- Empty `-force` value defaults to `version` mode
- Invalid syntax (e.g., `-force=,version`) fails with clear error

## Testing Strategy

### Unit Tests

Create `upgrade/force_test.go` with comprehensive ForceMode parsing tests:

- Valid single modes: `version`, `availability`, `readiness`, `all`
- Valid combinations: `version,readiness`, `availability,readiness`
- Default behavior: empty value defaults to `version`
- Invalid modes: `invalid`, `foo`, `bar`
- Edge cases: duplicate modes (`version,version`), whitespace handling
- `all` behavior: `all` alone, `all` combined with other modes
- Case sensitivity: should be case-insensitive or explicit about case

### Integration Tests

Manual testing for upgrade scenarios (no automated integration tests initially):
- Test each mode bypasses correct checks in real cluster
- Test combinations work correctly
- Test `all` mode bypasses everything

### Test Pattern

Follow existing Go testing pattern in the codebase:
- Table-driven tests for ForceMode parsing
- Use `testify` or standard Go testing (check existing test files for pattern)

## Implementation Notes

### Files to Create/Modify

1. Create `upgrade/force.go` - ForceMode type and flag parsing
2. Create `upgrade/force_test.go` - Unit tests
3. Modify `main.go` - Add flag definition, pass ForceMode to Manager
4. Modify `upgrade/manager.go` - Accept ForceMode, check before validations
5. Update `README.md` - Document `-force` flag usage

### Check Bypass Logic

Version checks to bypass:
- `version.ValidateTargetVersion()` - skip if `availability` or `all` mode active
- `version.NeedsUpgrade()` - skip if `version` or `all` mode active (assume needs upgrade)
- Node readiness checks in `validateUpgradePrerequisites()` - skip if `readiness` or `all` mode active
- All prerequisite checks - skip if `all` mode active

### Code Locations

Key locations to modify in `upgrade/manager.go`:
- Lines 122-127: Talos version availability validation
- Lines 139-143: Kubernetes version availability validation
- Lines 145-166: Kubernetes version matching checks
- Lines 823-856: `checkTalosUpgradeNeeded()` version matching
- Lines 703-743: `validateUpgradePrerequisites()` node readiness checks

## Success Criteria

- All force modes work as specified
- Invalid modes show helpful error messages
- Logging clearly indicates which checks are bypassed
- Unit tests cover all parsing scenarios
- No regression in normal upgrade behavior (when `-force` not used)
- README documents the feature with examples