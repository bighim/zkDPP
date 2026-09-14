package audit

import (
	"context"
	"fmt"
)

// CachedSource is created for ONE audit execution. Only records whose storage,
// calldata, receipt and canonical inclusion were verified are cached.
type CachedSource struct {
	*RPCSource
	records map[string]Record
	Hits    int
}

func NewCachedSource(s *RPCSource) *CachedSource {
	return &CachedSource{RPCSource: s, records: map[string]Record{}}
}
func (s *CachedSource) Record(ctx context.Context, id uint64, snap Snapshot) (Record, error) {
	key := fmt.Sprintf("%s:%s:%d", s.Ledger.Hex(), snap.BlockHash, id)
	if r, ok := s.records[key]; ok {
		s.Hits++
		return r, nil
	}
	r, e := s.RPCSource.Record(ctx, id, snap)
	if e == nil {
		s.records[key] = r
	}
	return r, e
}
