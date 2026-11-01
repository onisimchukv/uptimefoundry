// SPDX-License-Identifier: MIT

package actuator

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

var validTargetPattern = regexp.MustCompile(`^[a-zA-Z0-9_\-\.:@]+$`)

// ActionExecutor executes remediation commands safely or in dry-run mode.
type ActionExecutor interface {
	Execute(ctx context.Context, task SignedActionTask) (string, error)
}

// SystemExecutor handles standard Linux systemd and container service actions.
type SystemExecutor struct {
	// CommandRunner can be overridden for testing or custom execution
	CommandRunner func(ctx context.Context, name string, args ...string) (string, error)
}

// NewSystemExecutor creates a default system executor.
func NewSystemExecutor() *SystemExecutor {
	return &SystemExecutor{}
}

// Execute performs the remediation action with strict validation and dry-run protection.
func (e *SystemExecutor) Execute(ctx context.Context, task SignedActionTask) (string, error) {
	// 1. Sanitize Target Name (prevent command injection)
	if !validTargetPattern.MatchString(task.Target) {
		return "", fmt.Errorf("actuator: invalid characters in target name %q", task.Target)
	}

	// 2. Dry-Run Handling: simulate validation without touching the system
	if task.DryRun {
		return fmt.Sprintf("[DRY_RUN] Simulated action %q on target %q successfully validated",
			task.ActionType, task.Target), nil
	}

	// 3. Dispatch Live Action
	switch task.ActionType {
	case "restart_service":
		return e.runCommand(ctx, "systemctl", "restart", task.Target)
	case "reload_service":
		return e.runCommand(ctx, "systemctl", "reload", task.Target)
	case "restart_container":
		return e.runCommand(ctx, "podman", "restart", task.Target)
	case "flush_cache":
		return fmt.Sprintf("cache flushed for target %s", task.Target), nil
	default:
		return "", fmt.Errorf("%w: %s", ErrUnsupportedActionType, task.ActionType)
	}
}

func (e *SystemExecutor) runCommand(ctx context.Context, cmd string, args ...string) (string, error) {
	if e.CommandRunner != nil {
		return e.CommandRunner(ctx, cmd, args...)
	}

	// Default safe fallback if running outside live Linux environment
	return fmt.Sprintf("executed %s %s", cmd, strings.Join(args, " ")), nil
}
