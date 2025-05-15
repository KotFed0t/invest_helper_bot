package scheduler

import (
	"context"
	"log/slog"
	"runtime/debug"
	"time"

	"github.com/go-co-op/gocron/v2"
)

type taskFn func(ctx context.Context) error

type Scheduler struct {
	scheduler gocron.Scheduler
}

func New() *Scheduler {
	scheduler, err := gocron.NewScheduler()
	if err != nil {
		panic(err.Error())
	}
	return &Scheduler{scheduler: scheduler}
}

func (s *Scheduler) Start() {
	s.scheduler.Start()
}

func (s *Scheduler) Stop() {
	_ = s.scheduler.Shutdown()
}

func (s *Scheduler) createJob(jobDefinition gocron.JobDefinition, name string, fn taskFn, startImmediately bool) {
	opts := []gocron.JobOption{gocron.WithSingletonMode(gocron.LimitModeReschedule)}

	if startImmediately {
		opts = append(opts, gocron.WithStartAt(gocron.WithStartImmediately()))
	}

	_, err := s.scheduler.NewJob(
		jobDefinition,
		gocron.NewTask(s.taskWithRecover(fn, name)),
		opts...,
	)

	if err != nil {
		slog.Error("Scheduler creating job error", slog.String("jobName", name))
		panic(err.Error())
	}
}

func (s *Scheduler) NewIntervalJob(name string, fn taskFn, interval time.Duration, startImmediately bool) {
	s.createJob(gocron.DurationJob(interval), name, fn, startImmediately)
}

func (s *Scheduler) NewCrontabJob(name string, fn taskFn, crontab string, startImmediately bool) {
	s.createJob(gocron.CronJob(crontab, true), name, fn, startImmediately)
}

func (s *Scheduler) taskWithRecover(fn taskFn, jobName string) func(ctx context.Context) {
	return func(ctx context.Context) {
		defer func() {
			if r := recover(); r != nil {
				slog.Error(
					"Panic recovered in scheduler job",
					slog.String("jobName", jobName),
					slog.Any("panic", r),
					slog.String("stacktrace", string(debug.Stack())),
				)
			}
		}()

		slog.Info("job start", slog.String("jobName", jobName))

		err := fn(ctx)
		if err != nil {
			slog.Error("job failed", slog.String("jobName", jobName), slog.Any("error", err))
		} else {
			slog.Info("job completed", slog.String("jobName", jobName))
		}
	}
}
