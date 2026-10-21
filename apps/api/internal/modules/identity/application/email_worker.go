package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

const (
	emailLeaseDuration = 2 * time.Minute
	emailSendTimeout   = 30 * time.Second
	emailPoolInterval  = time.Second
	emailMaxAttempts   = 5
)

type EmailWorker struct {
	jobs   EmailOutboxJobRepository
	opener EmailPayloadOpener
	sender EmailSender
	logger *slog.Logger
}

func NewEmailWorker(jobs EmailOutboxJobRepository, opener EmailPayloadOpener, sender EmailSender, logger *slog.Logger) *EmailWorker {
	if logger == nil {
		logger = slog.Default()
	}

	return &EmailWorker{
		jobs:   jobs,
		opener: opener,
		sender: sender,
		logger: logger,
	}
}

func (w *EmailWorker) ProcessOne(ctx context.Context) (bool, error) {
	job, found, err := w.jobs.Claim(ctx, emailLeaseDuration, emailMaxAttempts)

	if err != nil {
		return false, fmt.Errorf("claim email job: %w", err)
	}

	if !found {
		return false, nil
	}

	// finalize database state even if the worker is
	// shutting down adn its main context was canceled
	finishCtx, cancelFinish := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)

	defer cancelFinish()

	allowed, err := w.jobs.CanSend(ctx, job)
	if err != nil {
		if retryErr := w.retryOrDead(finishCtx, job); retryErr != nil {
			return true, retryErr
		}

		return true, fmt.Errorf("check email eligibility: %w", err)
	}

	if !allowed {
		return true, w.jobs.MarkDead(ctx, job)
	}

	message, err := w.opener.Open(
		job.ID,
		job.Payload,
	)
	if err != nil {
		// a corrupted or undecryptable payload
		// cannot be fixed by retrying with the same key
		return true, w.jobs.MarkDead(finishCtx, job)
	}

	sendCtx, cancelSend := context.WithTimeout(
		ctx,
		emailSendTimeout,
	)

	sendErr := w.sender.SendVerification(
		sendCtx,
		message.To,
		message.Token,
		job.ID,
	)

	cancelSend()

	if sendErr != nil {
		// do not log provider errors blindly
		w.logger.Warn(
			"verification email delivery failed",
			"outbox_id", job.ID,
			"attempts", job.Attempts,
		)

		return true, w.retryOrDead(finishCtx, job)
	}

	if err := w.jobs.MarkSent(finishCtx, job); err != nil {
		return true, fmt.Errorf("mark verification email sent: %w", err)
	}

	return true, nil
}

func (w *EmailWorker) retryOrDead(ctx context.Context, job EmailOutboxJob) error {
	if job.Attempts >= emailMaxAttempts {
		return w.jobs.MarkDead(ctx, job)
	}

	next := time.Now().UTC().Add(emailRetryDelay(job.Attempts))

	return w.jobs.MarkRetry(ctx, job, next)
}

func emailRetryDelay(attempt int) time.Duration {
	delay := 5 * time.Second
	maxDelay := 15 * time.Minute

	for i := 1; i < attempt; i++ {
		if delay >= maxDelay/2 {
			return maxDelay
		}
		delay *= 2
	}

	return delay
}

func (w *EmailWorker) Run(ctx context.Context, concurrency int) error {
	if concurrency < 1 || concurrency > 20 {
		return errors.New("email worker concurrency must be between 1 and 20")
	}

	var wg sync.WaitGroup

	for i := 0; i < concurrency; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for {
				if ctx.Err() != nil {
					return
				}

				processed, err := w.ProcessOne(ctx)

				if err != nil && ctx.Err() == nil {
					w.logger.Error("email worker iteration failed", "err", err)
				}

				// if no job was available or an error occured
				// avoid a tight polling loop
				if !processed || err != nil {
					timer := time.NewTimer(emailPoolInterval)

					select {
					case <-ctx.Done():
						timer.Stop()
						return
					case <-timer.C:
					}
				}
			}
		}()
	}

	<-ctx.Done()
	wg.Wait()

	return nil
}
