// Package xcore provides cron job scheduling functionality.
//
// This package wraps the robfig/cron library to provide a simple interface
// for scheduling recurring tasks in the application.
package xcore

import (
	"fmt"
	"time"

	"github.com/robfig/cron/v3"
)

// CronJob represents a scheduled cron job.
type CronJob struct {
	Name  string
	Spec  string
	Func  func() error
	jobID cron.EntryID
	cron  *Cron
}

// Cron is a job scheduler for running recurring tasks.
// It supports panic recovery and configurable timezones.
type Cron struct {
	cron       *cron.Cron
	logger     *Logger
	jobs       map[cron.EntryID]*CronJob
	recoverPan bool
}

// NewCron creates a new Cron scheduler with the given configuration.
// If cfg is nil, default configuration is used (UTC timezone, panic recovery enabled).
func NewCron(cfg *CronConfig, logger *Logger) *Cron {
	recoverPan := true
	if cfg != nil && !cfg.RecoverPan {
		recoverPan = false
	}

	c := cron.New(cron.WithLocation(time.UTC))

	if cfg != nil && cfg.Timezone != "" {
		loc, err := time.LoadLocation(cfg.Timezone)
		if err == nil {
			c = cron.New(cron.WithLocation(loc))
		}
	}

	return &Cron{
		cron:       c,
		logger:     logger,
		jobs:       make(map[cron.EntryID]*CronJob),
		recoverPan: recoverPan,
	}
}

// AddJob adds a new cron job with the given name, spec, and function.
// The spec is a cron expression (e.g., "0 0 * * *" for daily at midnight).
// Returns the CronJob and any error.
func (c *Cron) AddJob(name, spec string, fn func() error) (*CronJob, error) {
	job := &CronJob{
		Name: name,
		Spec: spec,
		Func: fn,
		cron: c,
	}

	jobFunc := func() {
		if c.logger != nil {
			c.logger.Debug().Str("job", name).Msg("running cron job")
		}
		if err := fn(); err != nil {
			if c.logger != nil {
				c.logger.Error().Err(err).Str("job", name).Msg("cron job failed")
			}
		}
	}

	var wrappedFunc func()
	if c.recoverPan {
		wrappedFunc = func() {
			defer func() {
				if r := recover(); r != nil {
					if c.logger != nil {
						c.logger.Error().Interface("panic", r).Str("job", name).Msg("cron job panicked")
					}
				}
			}()
			jobFunc()
		}
	} else {
		wrappedFunc = jobFunc
	}

	jobID, err := c.cron.AddFunc(spec, wrappedFunc)

	if err != nil {
		return nil, fmt.Errorf("failed to add cron job: %w", err)
	}

	job.jobID = jobID
	c.jobs[jobID] = job

	if c.logger != nil {
		c.logger.Info().Str("job", name).Str("spec", spec).Msg("added cron job")
	}

	return job, nil
}

// AddFunc is an alias for AddJob.
func (c *Cron) AddFunc(name, spec string, fn func() error) (*CronJob, error) {
	return c.AddJob(name, spec, fn)
}

// Remove removes a cron job by its ID.
func (c *Cron) Remove(id cron.EntryID) {
	c.cron.Remove(id)
	delete(c.jobs, id)
}

// Start starts the cron scheduler. Jobs begin running according to their schedules.
func (c *Cron) Start() {
	c.cron.Start()
	if c.logger != nil {
		c.logger.Info().Msg("cron started")
	}
}

// Stop stops the cron scheduler gracefully.
// Waits for running jobs to complete.
func (c *Cron) Stop() {
	ctx := c.cron.Stop()
	<-ctx.Done()
	if c.logger != nil {
		c.logger.Info().Msg("cron stopped")
	}
}

// Entries returns all registered cron job entries.
func (c *Cron) Entries() []cron.Entry {
	return c.cron.Entries()
}

// Run triggers all jobs to run immediately (for testing purposes).
func (c *Cron) Run() {
	c.cron.Run()
}

// ListJobs returns all registered CronJob objects.
func (c *Cron) ListJobs() []*CronJob {
	jobs := make([]*CronJob, 0, len(c.jobs))
	for _, job := range c.jobs {
		jobs = append(jobs, job)
	}
	return jobs
}
