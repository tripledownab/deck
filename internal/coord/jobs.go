package coord

// The record of a spawned analysis: what it is, what it cost, and reading it
// back. Starting one is in analyse.go.

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/tripledownab/deck/internal/agent"
)

// JobState is how far a spawned analysis has got.
type JobState int

const (
	JobRunning JobState = iota
	JobDone
	JobFailed
)

func (s JobState) String() string {
	switch s {
	case JobDone:
		return "done"
	case JobFailed:
		return "failed"
	default:
		return "running"
	}
}

// maxJobs bounds a session's analyses, the way maxInbox bounds its mail and
// maxNotes bounds the log. Each record holds a full review, so a session that
// keeps asking would otherwise grow without limit. Dropping the oldest is
// safe: Spend is a running total kept separately, so a discarded record costs
// the reader an old answer, never the bill.
const maxJobs = 50

// Job is one spawned analysis and what it cost.
type Job struct {
	ID      string
	From    string // session id that asked
	Subject string // session name analysed
	State   JobState
	Started time.Time

	// Elapsed is how long the caller has waited while running, and what the
	// turn itself took once finished. One field, because a reader wants "how
	// long" in both cases and two clocks for one quantity invite a silent
	// change of meaning.
	Elapsed time.Duration

	Answer string
	Cost   float64
	Tokens agent.Tokens
	Err    string
}

// Analysis returns a job the caller started. Jobs are private to the session
// that asked: a review is work someone paid for, and the answer is theirs.
func (c *Coordinator) Analysis(sessionID, jobID string) (*Job, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	job, ok := c.jobs[jobID]
	if !ok || job.From != sessionID {
		return nil, fmt.Errorf("no analysis %q started by this session", jobID)
	}
	copied := *job
	// Elapsed is written when a run finishes, so a job still in flight would
	// otherwise report zero — the one case where the caller most wants to know
	// how long it has been waiting.
	if copied.State == JobRunning {
		copied.Elapsed = time.Since(copied.Started)
	}
	return &copied, nil
}

// Spend is what a session's spawned analyses have cost it so far.
//
// Per session rather than per project, and never written to disk. A per-run
// figure alone is hard to read — the same short turn measured at $0.012 and
// $0.237 depending on whether its context was read from cache or written to
// it — so the running total is what makes a pattern visible.
func (c *Coordinator) Spend(sessionID string) float64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.spend[sessionID]
}

// trimJobs drops a session's oldest analyses past maxJobs. Caller holds the
// lock.
func (c *Coordinator) trimJobs(sessionID string) {
	mine := make([]*Job, 0, len(c.jobs))
	for _, j := range c.jobs {
		if j.From == sessionID {
			mine = append(mine, j)
		}
	}
	if len(mine) <= maxJobs {
		return
	}
	sort.Slice(mine, func(i, j int) bool { return mine[i].Started.Before(mine[j].Started) })
	for _, j := range mine[:len(mine)-maxJobs] {
		delete(c.jobs, j.ID)
	}
}

// jobSeq numbers analyses. Sequential rather than random: an agent reads an id
// back to us, and a short one it can retype is worth more than an unguessable
// one for something scoped to a single session anyway.
var jobSeq struct {
	sync.Mutex
	n int
}

func newJobID() string {
	jobSeq.Lock()
	defer jobSeq.Unlock()
	jobSeq.n++
	return fmt.Sprintf("a%d", jobSeq.n)
}

// Analyses is what a session's spawned reviews cost and how many are still
// running, for the sidebar badge.
//
// Two numbers rather than the records themselves: the sidebar has room for a
// glyph and a figure, and returning fifty full reviews so the caller can count
// them would hand a renderer the whole answer text on every frame.
func (c *Coordinator) Analyses(sessionID string) (running int, spent float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, j := range c.jobs {
		if j.From == sessionID && j.State == JobRunning {
			running++
		}
	}
	return running, c.spend[sessionID]
}
