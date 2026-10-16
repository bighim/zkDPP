#!/usr/bin/env python3
"""Aggregate official repeats without dropping unfavorable cases or warm caches."""
from pathlib import Path
from collections import defaultdict
import csv
import json
import statistics

ROOT = Path(__file__).resolve().parent
raw = json.loads((ROOT / 'raw.json').read_text())
all_rows = raw['Rows']
rows = [r for r in all_rows if r['run'] > 0]
assert raw['MeasuredRuns'] == 5 and raw['Warmups'] == 1
assert len(all_rows) == 1224 and len(rows) == 1020
assert all(r['matched'] for r in all_rows)
assert all(r['ancestors'] == 100 for r in all_rows if r['phase'] != 'prepare')

def stats(values):
    return dict(median=statistics.median(values), minimum=min(values), maximum=max(values))

def ms(values):
    return stats([x / 1_000_000 for x in values])

def uniform(values):
    assert all(x == values[0] for x in values), values
    return values[0]

keys = ('size', 'layout', 'transport', 'strategy')
groups = defaultdict(list)
for row in rows:
    groups[tuple(row[k] for k in keys)].append(row)

summaries = []
curves = []
for key, rr in sorted(groups.items()):
    base = dict(zip(keys, key))
    phases = {phase: [r for r in rr if r['phase'] == phase]
              for phase in ('prepare', 'first', 'repeat_same', 'repeat_last')}
    assert all(len(rs) == 5 for rs in phases.values())
    first, last = phases['first'], phases['repeat_last']
    result = dict(base)
    result.update(
        cold_first_ms=ms([r['cumulative_ns'] + r['catalogue_ns'] for r in first]),
        common_catalogue_ms=ms([r['catalogue_ns'] for r in first]),
        preparation_ms=ms([r['ns'] for r in phases['prepare']]),
        first_query_only_ms=ms([r['ns'] for r in first]),
        same_seed_cached_ms=ms([r['ns'] for r in phases['repeat_same']]),
        complete_session_ms=ms([r['cumulative_ns'] + r['catalogue_ns'] for r in last]),
        first_work=uniform([r['cumulative'] for r in first]),
        final_work=uniform([r['cumulative'] for r in last]),
        catalogue_work=uniform([r['catalogue_metrics'] for r in first]),
        first_cache_records=uniform([r['cache_records'] for r in first]),
        first_cache_plain=uniform([r['cache_plain'] for r in first]),
        first_map_entries=uniform([r['map_entries'] for r in first]),
        first_logical_cache_bytes=uniform([r['cache_logical_bytes'] for r in first]),
        final_cache_records=uniform([r['cache_records'] for r in last]),
        final_map_entries=uniform([r['map_entries'] for r in last]),
        final_logical_cache_bytes=uniform([r['cache_logical_bytes'] for r in last]),
        final_retained_heap_bytes=stats([r.get('retained_heap_bytes', 0) for r in last]),
        distinct_seeds=uniform([r['distinct_seeds'] for r in last]),
    )
    for r in phases['repeat_same'] + phases['repeat_last']:
        assert r['delta']['Decryptions'] == 0 and r['delta']['Requests'] == 0
    for q in range(1, key[0] // 100 + 1):
        qs = [r for r in rr if r['distinct_seeds'] == q and r['phase'] in ('first', 'new_seed')]
        assert len(qs) == 5
        curve = dict(base, distinct_seeds=q,
                     cumulative_ms=ms([r['cumulative_ns'] + r['catalogue_ns'] for r in qs]),
                     cumulative_work=uniform([r['cumulative'] for r in qs]))
        curves.append(curve)
    # P always decrypts 99 newly encountered non-Entry records per disjoint flow.
    if key[-1] == 'parents':
        assert result['first_work']['Decryptions'] == 99
        assert result['final_work']['Decryptions'] == 99 * (key[0] // 100)
    elif key[-1] == 'full_map':
        assert result['first_work']['Decryptions'] == key[0]
    else:
        expected = 99 if key[1] == 'contiguous' else key[0] - (key[0] // 100)
        assert result['first_work']['Decryptions'] == expected
    summaries.append(result)

crossovers = []
for size in (100, 1000):
    for layout in ('contiguous', 'interleaved'):
        for transport in ('local_preloaded', 'http_loopback'):
            case = dict(size=size, layout=layout, transport=transport)
            for baseline in ('full_map', 'lazy_map'):
                q_win = None
                for q in range(1, size // 100 + 1):
                    def value(strategy):
                        matching = [r for r in curves if all(r[k] == v for k, v in case.items())
                                    and r['strategy'] == strategy and r['distinct_seeds'] == q]
                        assert len(matching) == 1
                        return matching[0]['cumulative_ms']['median']
                    if value(baseline) < value('parents'):
                        q_win = q
                        break
                crossovers.append(dict(case, baseline=baseline,
                                       first_measured_distinct_seed_count_with_lower_median=q_win))

summary = dict(
    source='raw.json', measured_rows=len(rows), measured_query_checks=sum(r['phase'] != 'prepare' for r in rows),
    aggregation='Median/min/max of five official runs; run=0 excluded. Cold and cumulative totals include common catalogue. Key recovery is common, reported separately. Cumulative multi-seed curves include the one same-seed repeat after seed 0.',
    cases=raw['Cases'], summaries=summaries, cumulative_curves=curves, crossovers=crossovers,
    correctness='All 900 official query results and 180 warmup query results matched exact 100-object ancestor sets. Native encrypt/decrypt and public-input/NF correspondence also checked for every fixture record.',
    heap_limit='Heap delta measured after final session GC is process-level and includes HTTP buffers and retained measurement rows; cache_logical_bytes is a deterministic cache payload/accounting proxy excluding Go map/allocator overhead. Neither is peak process RSS.',
)
(ROOT / 'summary.json').write_text(json.dumps(summary, ensure_ascii=False, indent=2) + '\n')

flat_rows = []
for r in all_rows:
    record = {k: v for k, v in r.items() if k not in ('delta', 'cumulative', 'catalogue_metrics')}
    record['excluded_warmup'] = r['run'] == 0
    for group in ('delta', 'cumulative', 'catalogue_metrics'):
        record.update({f'{group}_{k}': v for k, v in r[group].items()})
    flat_rows.append(record)
columns = sorted(set().union(*(set(r) for r in flat_rows)))
with (ROOT / 'raw.csv').open('w', newline='') as f:
    writer = csv.DictWriter(f, fieldnames=columns)
    writer.writeheader(); writer.writerows(flat_rows)

fields = list(keys) + ['cold_median_ms', 'cold_min_ms', 'cold_max_ms', 'prepare_median_ms',
    'query_only_median_ms', 'same_seed_cached_median_ms', 'complete_session_median_ms',
    'distinct_seeds', 'first_decryptions', 'first_fields', 'first_record_reads',
    'first_producer_reads', 'first_requests', 'first_response_bytes',
    'first_logical_cache_bytes', 'final_logical_cache_bytes']
with (ROOT / 'comparison.csv').open('w', newline='') as f:
    writer = csv.DictWriter(f, fieldnames=fields); writer.writeheader()
    for s in summaries:
        r = {k: s[k] for k in keys}
        r.update(cold_median_ms=s['cold_first_ms']['median'], cold_min_ms=s['cold_first_ms']['minimum'],
                 cold_max_ms=s['cold_first_ms']['maximum'], prepare_median_ms=s['preparation_ms']['median'],
                 query_only_median_ms=s['first_query_only_ms']['median'],
                 same_seed_cached_median_ms=s['same_seed_cached_ms']['median'],
                 complete_session_median_ms=s['complete_session_ms']['median'], distinct_seeds=s['distinct_seeds'],
                 first_logical_cache_bytes=s['first_logical_cache_bytes'], final_logical_cache_bytes=s['final_logical_cache_bytes'])
        for field, source in [('first_decryptions','Decryptions'), ('first_fields','Fields'),
                              ('first_record_reads','RecordReads'), ('first_producer_reads','ProducerReads'),
                              ('first_requests','Requests'), ('first_response_bytes','ResponseBytes')]:
            r[field] = s['first_work'][source]
        writer.writerow(r)

print('Validated 24 case/strategy groups, 900 official exact-set checks; wrote summary.json, raw.csv, comparison.csv.')
for c in crossovers:
    if c['size'] == 1000:
        print(c)
