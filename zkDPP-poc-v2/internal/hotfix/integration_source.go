package hotfix

import (
	"context"
	"fmt"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/v2audit"
)

// Test-only faults mask one response after the underlying real ledger action.
// Embedding preserves all Source methods; optional interfaces remain explicit.
type integratedFaultSource struct {
	*v2audit.RPCSource
	mode                           string
	selected                       v2audit.Frontier
	targets, checkpoints           int
	final                          bool
	last                           *v2audit.RPCTransaction
	injections, receiptValidations int
	observations                   []map[string]any
}

func (s *integratedFaultSource) Checkpoint(ctx context.Context) (v2audit.Snapshot, error) {
	h, err := s.RPCSource.Checkpoint(ctx)
	s.checkpoints++
	s.final = s.checkpoints > s.targets
	s.observations = append(s.observations, map[string]any{"stage": "checkpoint", "snapshot": h, "final": s.final})
	return h, err
}
func (s *integratedFaultSource) Status(ctx context.Context, t uint8, id [32]byte, h v2audit.Snapshot) (uint8, bool, error) {
	if s.mode == "final-status-unavailable" && s.final && t == s.selected.Ref.ObjectType && id == s.selected.SpendValue {
		s.injections++
		s.observations = append(s.observations, map[string]any{"stage": "final-status", "snapshot": h, "type": t, "id": id, "injected": true})
		return 0, false, fmt.Errorf("injected final V_Q status unavailable")
	}
	status, spent, err := s.RPCSource.Status(ctx, t, id, h)
	s.observations = append(s.observations, map[string]any{"stage": "status", "snapshot": h, "type": t, "id": id, "status": status, "spent": spent, "final": s.final})
	return status, spent, err
}
func (s *integratedFaultSource) Freeze(ctx context.Context, t uint8, id [32]byte) error {
	err := s.RPCSource.Freeze(ctx, t, id)
	s.last = s.RPCSource.LastTransaction()
	if err == nil && s.mode == "receipt-uncertain" && t == s.selected.Ref.ObjectType && id == s.selected.SpendValue {
		s.injections++
		// Only the hash is available to the client. Confirmed receipt data stays
		// in RPCSource and the separately saved ground-truth observations.
		s.last = &v2audit.RPCTransaction{Hash: s.last.Hash, State: "UNCERTAIN", Error: "injected V_Q receipt unavailable", CalldataBytes: s.last.CalldataBytes}
		return fmt.Errorf("injected V_Q receipt unavailable")
	}
	return err
}
func (s *integratedFaultSource) LastTransaction() *v2audit.RPCTransaction {
	if s.last == nil {
		return nil
	}
	v := *s.last
	return &v
}
func (s *integratedFaultSource) ValidateReceipt(ctx context.Context, tx v2audit.RPCTransaction, h v2audit.Snapshot) error {
	s.receiptValidations++
	return s.RPCSource.ValidateReceipt(ctx, tx, h)
}

var _ v2audit.Source = (*integratedFaultSource)(nil)
var _ interface {
	LastTransaction() *v2audit.RPCTransaction
} = (*integratedFaultSource)(nil)
var _ interface {
	ValidateReceipt(context.Context, v2audit.RPCTransaction, v2audit.Snapshot) error
} = (*integratedFaultSource)(nil)
