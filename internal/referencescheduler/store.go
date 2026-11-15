// Copyright (c) 2026 The cordum Agent Protocol Authors
// SPDX-License-Identifier: Apache-2.0

package referencescheduler

import (
	"cmp"
	"slices"
	"strings"
	"sync"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	"google.golang.org/protobuf/proto"
)

// jobKey is the tenant-qualified job key: the same job_id under two tenants is two jobs.
type jobKey struct{ tenant, job string }

// record is one stored job. Records are copy-on-write: a transition swaps in a
// new record, so a refused call leaves the stored one untouched (R1).
type record struct {
	job     Job
	request *agentv1.JobRequest // the admitted request; never mutated after insert
}

// store is the tenant-scoped in-memory job table. Its mutex guards map access
// and pure state computation only: no clock read, ID source, callback or
// channel send runs while it is held.
type store struct {
	mu   sync.Mutex
	jobs map[jobKey]*record
}

// insert stores rec under a new key. For an existing key it returns that job's
// snapshot when the admitted request is proto.Equal to rec.request, and
// ErrJobConflict otherwise; neither case changes anything.
func (st *store) insert(k jobKey, rec *record) (job Job, inserted bool, err error) {
	st.mu.Lock()
	defer st.mu.Unlock()
	if cur, ok := st.jobs[k]; ok {
		if !proto.Equal(cur.request, rec.request) {
			return Job{}, false, ErrJobConflict
		}
		return cloneJob(cur.job), false, nil
	}
	st.jobs[k] = rec
	return cloneJob(rec.job), true, nil
}

// update is the compare-and-swap transition: apply edits a copy of the stored
// job, and the copy replaces it only when apply reports commit. The snapshot is
// the job after the call; a refusal other than ErrTerminal returns Job{}.
func (st *store) update(k jobKey, apply func(j *Job, req *agentv1.JobRequest) (commit bool, err error)) (Job, error) {
	st.mu.Lock()
	defer st.mu.Unlock()
	cur, ok := st.jobs[k]
	if !ok {
		return Job{}, ErrJobNotFound
	}
	next := cloneJob(cur.job)
	commit, err := apply(&next, cur.request)
	if commit {
		st.jobs[k] = &record{job: next, request: cur.request}
		return cloneJob(next), err
	}
	if err != nil && err != ErrTerminal {
		return Job{}, err
	}
	return cloneJob(cur.job), err
}

// get returns a deep snapshot of the job under k.
func (st *store) get(k jobKey) (Job, error) {
	return st.update(k, func(*Job, *agentv1.JobRequest) (bool, error) { return false, nil })
}

// sweep applies change to every job that due selects and returns the changed
// snapshots sorted by (TenantID, JobID).
func (st *store) sweep(due func(*Job) bool, change func(*Job)) []Job {
	st.mu.Lock()
	defer st.mu.Unlock()
	var out []Job
	for k, cur := range st.jobs {
		if !due(&cur.job) {
			continue
		}
		next := cloneJob(cur.job)
		change(&next)
		st.jobs[k] = &record{job: next, request: cur.request}
		out = append(out, cloneJob(next))
	}
	slices.SortFunc(out, func(a, b Job) int {
		return cmp.Or(strings.Compare(a.TenantID, b.TenantID), strings.Compare(a.JobID, b.JobID))
	})
	return out
}

// cloneJob deep-copies j so that no snapshot shares memory with stored state.
func cloneJob(j Job) Job {
	j.Dispatch, j.Result = clone(j.Dispatch), clone(j.Result)
	if j.Artifacts != nil {
		refs := make([]*agentv1.ResourceRef, len(j.Artifacts))
		for i, ref := range j.Artifacts {
			refs[i] = clone(ref)
		}
		j.Artifacts = refs
	}
	return j
}

// clone deep-copies a message; a typed nil stays a typed nil.
func clone[M proto.Message](m M) M { return proto.Clone(m).(M) }
