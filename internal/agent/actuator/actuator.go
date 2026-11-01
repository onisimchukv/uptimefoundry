// SPDX-License-Identifier: MIT

package actuator

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// ActuatorConfig configures the remediation actuator service.
type ActuatorConfig struct {
	AllowedActions map[string]bool // Local policy ceiling allowlist
	DryRunDefault  bool
}

// Actuator coordinates signature validation, local policy verification, execution, and audit logging.
type Actuator struct {
	cfg      ActuatorConfig
	verifier TaskVerifier
	executor ActionExecutor
	audit    *AuditLogger
	logger   *slog.Logger
}

// NewActuator constructs an Actuator service instance.
func NewActuator(
	cfg ActuatorConfig,
	verifier TaskVerifier,
	executor ActionExecutor,
	audit *AuditLogger,
	logger *slog.Logger,
) *Actuator {
	if executor == nil {
		executor = NewSystemExecutor()
	}
	return &Actuator{
		cfg:      cfg,
		verifier: verifier,
		executor: executor,
		audit:    audit,
		logger:   logger,
	}
}

// Handle processes an incoming remediation task request.
func (a *Actuator) Handle(ctx context.Context, task SignedActionTask) (ExecutionResult, error) {
	start := time.Now()
	res := ExecutionResult{
		TaskID:     task.TaskID,
		DryRun:     task.DryRun || a.cfg.DryRunDefault,
		ExecutedAt: start,
	}

	// 1. Verify Cryptographic Signature and Timestamps
	if err := a.verifier.Verify(task); err != nil {
		a.logger.Warn("remediation task signature rejected",
			slog.String("task_id", task.TaskID),
			slog.Any("error", err),
		)
		if a.audit != nil {
			_, _ = a.audit.Log(task, "rejected_signature", err.Error())
		}
		res.Success = false
		res.ErrorMessage = err.Error()
		res.DurationMs = time.Since(start).Milliseconds()
		return res, err
	}

	// 2. Enforce Local Security Policy Ceiling
	if !a.cfg.AllowedActions[task.ActionType] {
		err := fmt.Errorf("%w: action %q is not in local policy allowlist",
			ErrActionDeniedByCeiling, task.ActionType)
		a.logger.Warn("remediation task rejected by local policy ceiling",
			slog.String("task_id", task.TaskID),
			slog.String("action_type", task.ActionType),
		)
		if a.audit != nil {
			_, _ = a.audit.Log(task, "denied_by_policy", err.Error())
		}
		res.Success = false
		res.ErrorMessage = err.Error()
		res.DurationMs = time.Since(start).Milliseconds()
		return res, err
	}

	// 3. Execute Action (or Dry-Run simulation)
	output, err := a.executor.Execute(ctx, task)
	res.DurationMs = time.Since(start).Milliseconds()

	if err != nil {
		a.logger.Error("remediation action failed",
			slog.String("task_id", task.TaskID),
			slog.Any("error", err),
		)
		if a.audit != nil {
			_, _ = a.audit.Log(task, "failed", err.Error())
		}
		res.Success = false
		res.ErrorMessage = err.Error()
		return res, err
	}

	// 4. Record Successful Execution in Audit Log
	outcome := "executed"
	if res.DryRun {
		outcome = "dry_run"
	}

	if a.audit != nil {
		_, _ = a.audit.Log(task, outcome, "")
	}

	res.Success = true
	res.Output = output
	a.logger.Info("remediation task completed successfully",
		slog.String("task_id", task.TaskID),
		slog.String("action_type", task.ActionType),
		slog.Bool("dry_run", res.DryRun),
	)

	return res, nil
}
