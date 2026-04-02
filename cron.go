package xcore

import (
	"fmt"
	"time"

	"github.com/robfig/cron/v3"
)

type CronJob struct {
	Name  string
	Spec  string
	Func  func() error
	jobID cron.EntryID
	cron  *Cron
}

type Cron struct {
	cron       *cron.Cron
	logger     *Logger
	jobs       map[cron.EntryID]*CronJob
	recoverPan bool
}

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

func (c *Cron) AddFunc(name, spec string, fn func() error) (*CronJob, error) {
	return c.AddJob(name, spec, fn)
}

func (c *Cron) Remove(id cron.EntryID) {
	c.cron.Remove(id)
	delete(c.jobs, id)
}

func (c *Cron) Start() {
	c.cron.Start()
	if c.logger != nil {
		c.logger.Info().Msg("cron started")
	}
}

func (c *Cron) Stop() {
	ctx := c.cron.Stop()
	<-ctx.Done()
	if c.logger != nil {
		c.logger.Info().Msg("cron stopped")
	}
}

func (c *Cron) Entries() []cron.Entry {
	return c.cron.Entries()
}

func (c *Cron) Run() {
	c.cron.Run()
}

func (c *Cron) ListJobs() []*CronJob {
	jobs := make([]*CronJob, 0, len(c.jobs))
	for _, job := range c.jobs {
		jobs = append(jobs, job)
	}
	return jobs
}
