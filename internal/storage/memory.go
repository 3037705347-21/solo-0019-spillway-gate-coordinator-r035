package storage

import (
	"context"
	"sync"
	"time"

	"example.com/spillway-gate-coordinator/internal/domain"
)

type MemoryRepository struct {
	mu        sync.RWMutex
	gates     map[string]domain.Gate
	revisions map[string][]domain.LimitRevision
	commands  map[string]domain.GateCommand
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		gates:     make(map[string]domain.Gate),
		revisions: make(map[string][]domain.LimitRevision),
		commands:  make(map[string]domain.GateCommand),
	}
}

func (m *MemoryRepository) CreateGate(_ context.Context, gate domain.Gate, baseline domain.LimitRevision) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.gates[gate.ID]; exists {
		return domain.Conflict("duplicate_gate_id", "gate id already exists")
	}
	m.gates[gate.ID] = gate
	m.revisions[gate.ID] = []domain.LimitRevision{baseline}
	return nil
}

func (m *MemoryRepository) Gate(_ context.Context, id string) (domain.Gate, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	gate, exists := m.gates[id]
	if !exists {
		return domain.Gate{}, domain.NotFound("gate_not_found", "gate id was not found")
	}
	return gate, nil
}

func (m *MemoryRepository) ReviseGateLimit(
	_ context.Context,
	gateID string,
	revisionID string,
	input domain.ReviseLimitInput,
	now time.Time,
) (domain.Gate, domain.LimitRevision, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	gate, exists := m.gates[gateID]
	if !exists {
		return domain.Gate{}, domain.LimitRevision{}, domain.NotFound("gate_not_found", "gate id was not found")
	}

	previous, sequence, err := gate.ReviseLimit(input.NewMaxAperture, now)
	if err != nil {
		return domain.Gate{}, domain.LimitRevision{}, err
	}

	revision, err := domain.NewLimitRevision(
		revisionID,
		gate.ID,
		sequence,
		previous,
		gate.MaxAperture,
		input.Basis,
		input.Reason,
		input.RequestedBy,
		now,
	)
	if err != nil {
		return domain.Gate{}, domain.LimitRevision{}, err
	}

	m.gates[gate.ID] = gate
	m.revisions[gate.ID] = append(m.revisions[gate.ID], revision)
	return gate, revision, nil
}

func (m *MemoryRepository) ListLimitRevisions(_ context.Context, gateID string) ([]domain.LimitRevision, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, exists := m.gates[gateID]; !exists {
		return nil, domain.NotFound("gate_not_found", "gate id was not found")
	}
	stored := m.revisions[gateID]
	result := make([]domain.LimitRevision, len(stored))
	copy(result, stored)
	return result, nil
}

func (m *MemoryRepository) CreateCommand(_ context.Context, command domain.GateCommand) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	gate, exists := m.gates[command.GateID]
	if !exists {
		return domain.NotFound("gate_not_found", "gate id was not found")
	}
	if gate.Status == domain.GateFaulted {
		return domain.Conflict("gate_faulted", "faulted gates cannot receive new commands")
	}
	if command.StartingAperture != gate.Aperture {
		return domain.Conflict("gate_aperture_changed", "gate aperture changed before the command was stored")
	}
	if command.LimitRevision != gate.LimitRevision {
		return domain.Conflict("gate_limit_changed", "gate aperture limit changed before the command was stored")
	}
	if command.TargetAperture > gate.MaxAperture {
		return domain.Invalid("invalid_target_aperture", "target aperture is outside the gate limit")
	}
	if _, exists := m.commands[command.ID]; exists {
		return domain.Conflict("duplicate_command_id", "command id already exists")
	}

	m.commands[command.ID] = command
	return nil
}

func (m *MemoryRepository) Command(_ context.Context, id string) (domain.GateCommand, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	command, exists := m.commands[id]
	if !exists {
		return domain.GateCommand{}, domain.NotFound("command_not_found", "command id was not found")
	}
	return cloneCommand(command), nil
}

// CommandDetail reads the command and its gate under the same read lock, so
// both snapshots describe one consistent repository state.
func (m *MemoryRepository) CommandDetail(_ context.Context, id string) (domain.GateCommand, domain.Gate, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	command, exists := m.commands[id]
	if !exists {
		return domain.GateCommand{}, domain.Gate{}, domain.NotFound("command_not_found", "command id was not found")
	}
	gate, exists := m.gates[command.GateID]
	if !exists {
		return domain.GateCommand{}, domain.Gate{}, domain.NotFound("gate_not_found", "gate id was not found")
	}
	return cloneCommand(command), gate, nil
}

func (m *MemoryRepository) ReviewCommand(
	_ context.Context,
	commandID string,
	verdict domain.InterlockVerdict,
	verifier string,
	note string,
	acknowledgedRevision int64,
	token string,
	now time.Time,
) (domain.GateCommand, domain.Gate, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	command, exists := m.commands[commandID]
	if !exists {
		return domain.GateCommand{}, domain.Gate{}, domain.NotFound("command_not_found", "command id was not found")
	}
	gate, exists := m.gates[command.GateID]
	if !exists {
		return domain.GateCommand{}, domain.Gate{}, domain.NotFound("gate_not_found", "gate id was not found")
	}

	// The decision is only valid against the gate's current limit. A stale
	// basis must be re-confirmed; an infeasible target can never clear.
	if verdict != domain.InterlockClear && verdict != domain.InterlockBlocked {
		return domain.GateCommand{}, domain.Gate{}, domain.Invalid("invalid_interlock_verdict", "verdict must be clear or blocked")
	}
	if err := command.ValidateReviewBasis(
		verdict,
		acknowledgedRevision,
		gate.MaxAperture,
		gate.LimitRevision,
	); err != nil {
		return domain.GateCommand{}, domain.Gate{}, err
	}

	decision, err := domain.NewInterlockDecision(string(verdict), verifier, note, gate.LimitRevision, now)
	if err != nil {
		return domain.GateCommand{}, domain.Gate{}, err
	}

	nextCommand := cloneCommand(command)
	if err := nextCommand.Review(decision, token, now); err != nil {
		return domain.GateCommand{}, domain.Gate{}, err
	}

	nextGate := gate
	if decision.IsClear() {
		if err := nextGate.Reserve(nextCommand.ID, now); err != nil {
			return domain.GateCommand{}, domain.Gate{}, err
		}
	}

	m.commands[nextCommand.ID] = nextCommand
	m.gates[nextGate.ID] = nextGate
	return cloneCommand(nextCommand), nextGate, nil
}

func (m *MemoryRepository) ExecuteCommand(
	_ context.Context,
	commandID string,
	token string,
	operator string,
	simulateFault bool,
	now time.Time,
) (domain.GateCommand, domain.Gate, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	command, exists := m.commands[commandID]
	if !exists {
		return domain.GateCommand{}, domain.Gate{}, domain.NotFound("command_not_found", "command id was not found")
	}
	gate, exists := m.gates[command.GateID]
	if !exists {
		return domain.GateCommand{}, domain.Gate{}, domain.NotFound("gate_not_found", "gate id was not found")
	}

	nextCommand := cloneCommand(command)
	if err := nextCommand.BeginExecution(token, operator, now); err != nil {
		return domain.GateCommand{}, domain.Gate{}, err
	}

	nextGate := gate
	if err := nextGate.StartMovement(nextCommand.ID, now); err != nil {
		return domain.GateCommand{}, domain.Gate{}, err
	}

	if simulateFault {
		if err := nextGate.Settle(nextCommand.TargetAperture, true, now); err != nil {
			return domain.GateCommand{}, domain.Gate{}, err
		}
		if err := nextCommand.Abort("simulated_gate_fault", now); err != nil {
			return domain.GateCommand{}, domain.Gate{}, err
		}
	} else {
		if err := nextGate.Settle(nextCommand.TargetAperture, false, now); err != nil {
			return domain.GateCommand{}, domain.Gate{}, err
		}
		if err := nextCommand.Complete(now); err != nil {
			return domain.GateCommand{}, domain.Gate{}, err
		}
	}

	m.commands[nextCommand.ID] = nextCommand
	m.gates[nextGate.ID] = nextGate
	return cloneCommand(nextCommand), nextGate, nil
}

func cloneCommand(command domain.GateCommand) domain.GateCommand {
	if command.Interlock == nil {
		return command
	}
	decision := *command.Interlock
	command.Interlock = &decision
	return command
}
