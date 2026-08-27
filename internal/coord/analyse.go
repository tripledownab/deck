package coord

// Starting a spawned analysis: what the reviewer is told, and how its run is
// bounded. The record it produces is in jobs.go.
//
// The caller does not wait. A review of a real diff outlasts the tool timeout
// of whatever asked for it, and that timeout belongs to the calling agent
// rather than to us, so a synchronous answer would be spent money nobody
// receives. Analyse returns a handle and Analysis collects it later.

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// jobTimeout bounds a spawned run. Long enough for a review of a substantial
// diff, short enough that a wedged process is not still billing an hour later.
const jobTimeout = 10 * time.Minute

// reviewPrompt is what the spawned agent is given. It receives the diff in the
// prompt and no tools at all, so everything it needs to answer has to be here.
const reviewPrompt = `You are reviewing work done by another agent on this project.
You cannot run anything or read any file: the change is reproduced in full below.

Session: %s — %s
Measured from: %s

Files changed:
%s

Patch:
%s

%s`

// defaultQuestion is used when the caller asks for a review without saying
// what it wants to know.
const defaultQuestion = "What is wrong, risky or incomplete in this change? " +
	"Be specific and cite the lines you mean. If it looks sound, say so briefly."

// Analyse spawns an agent to review a sibling's work and returns immediately
// with a handle.
//
// The reviewer is handed the diff and given no coordination tools: it has
// nothing to ask anyone, so wiring it to this server would be surface with no
// caller. It runs in plan mode, which answers a question but refuses to act,
// so a review cannot become an edit.
func (c *Coordinator) Analyse(sessionID, target, question string) (*Job, error) {
	found, w, err := c.workOf(sessionID, target)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(question) == "" {
		question = defaultQuestion
	}
	prompt := fmt.Sprintf(reviewPrompt,
		found.Name, found.Title, w.Base, w.Stat, w.Patch, question)

	job := &Job{
		ID: newJobID(), From: sessionID, Subject: found.Name,
		State: JobRunning, Started: time.Now(),
	}
	c.mu.Lock()
	c.jobs[job.ID] = job
	c.trimJobs(sessionID)
	dir := found.Dir
	c.mu.Unlock()

	go c.runAnalysis(job, dir, prompt)

	// A copy, not the live record. The mutex guards the coordinator's own
	// access to a job, not the caller's: handing back the pointer lets the
	// caller read fields the spawned goroutine is still writing.
	c.mu.Lock()
	copied := *job
	c.mu.Unlock()
	return &copied, nil
}

// runAnalysis performs the spawned turn and records what it cost.
//
// A turn that ran and refused is a result carrying its own cost, so the total
// counts it. An error here means the opposite: no envelope came back, so there
// is no accounting to add and the job records why rather than inventing a
// figure.
func (c *Coordinator) runAnalysis(job *Job, dir, prompt string) {
	// Derived from the coordinator's life, so Close stops this run. The
	// timeout then bounds a run within that lifetime rather than standing in
	// for one.
	ctx, cancel := context.WithTimeout(c.life, jobTimeout)
	defer cancel()

	run, err := c.spawn(ctx, dir, prompt)

	c.mu.Lock()
	defer c.mu.Unlock()
	if err != nil {
		// No envelope came back, so the CLI reported no duration either; our
		// own measurement is all there is.
		job.Elapsed = time.Since(job.Started)
		job.State, job.Err = JobFailed, err.Error()
		return
	}
	job.Elapsed = run.Took
	job.Cost, job.Tokens = run.CostUSD, run.Tokens
	c.spend[job.From] += run.CostUSD
	if run.Failed() {
		job.State, job.Err = JobFailed, run.Failure
		return
	}
	job.State, job.Answer = JobDone, run.Text
}
