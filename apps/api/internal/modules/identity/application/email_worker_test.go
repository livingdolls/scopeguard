package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

type fakeEmailJobRepository struct {
	job        EmailOutboxJob
	claimed    bool
	allowed    bool
	finalState string
	nextRetry  time.Time
}

func (f *fakeEmailJobRepository) Claim(
	ctx context.Context,
	lease time.Duration,
	maxAttempts int,
) (EmailOutboxJob, bool, error) {
	if f.claimed {
		return EmailOutboxJob{}, false, nil
	}

	f.claimed = true
	return f.job, true, nil
}

func (f *fakeEmailJobRepository) CanSend(
	ctx context.Context,
	job EmailOutboxJob,
) (bool, error) {
	return f.allowed, nil
}

func (f *fakeEmailJobRepository) MarkSent(
	ctx context.Context,
	job EmailOutboxJob,
) error {
	f.finalState = "sent"
	return nil
}

func (f *fakeEmailJobRepository) MarkDead(
	ctx context.Context,
	job EmailOutboxJob,
) error {
	f.finalState = "dead"
	return nil
}

func (f *fakeEmailJobRepository) MarkRetry(
	ctx context.Context,
	job EmailOutboxJob,
	availableAt time.Time,
) error {
	f.finalState = "pending"
	f.nextRetry = availableAt
	return nil
}

type fakeEmailOpener struct {
	message VerificationEmail
	err     error
}

func (f *fakeEmailOpener) Open(
	eventID uuid.UUID,
	payload EncryptedEmailPayload,
) (VerificationEmail, error) {
	return f.message, f.err
}

type fakeEmailSender struct {
	calls int
	to    string
	id    uuid.UUID
	err   error
}

func (f *fakeEmailSender) SendVerification(
	ctx context.Context,
	to string,
	token string,
	deliveryID uuid.UUID,
) error {
	f.calls++
	f.to = to
	f.id = deliveryID
	return f.err
}

func newEmailWorkerFixture(
	attempts int,
) (
	*EmailWorker,
	*fakeEmailJobRepository,
	*fakeEmailSender,
) {
	jobs := &fakeEmailJobRepository{
		job: EmailOutboxJob{
			ID:       uuid.New(),
			LeaseID:  uuid.New(),
			Attempts: attempts,
		},
		allowed: true,
	}

	opener := &fakeEmailOpener{
		message: VerificationEmail{
			To:    "user@example.com",
			Token: "secret-verification-token",
		},
	}

	sender := &fakeEmailSender{}

	worker := NewEmailWorker(
		jobs, opener, sender, nil,
	)

	return worker, jobs, sender
}

func TestEmailWorkerSuccess(t *testing.T) {
	worker, jobs, sender := newEmailWorkerFixture(1)

	processed, err := worker.ProcessOne(
		context.Background(),
	)
	if err != nil {
		t.Fatal(err)
	}

	if !processed || jobs.finalState != "sent" {
		t.Fatal("expected email job to be marked sent")
	}

	if sender.calls != 1 ||
		sender.to != "user@example.com" ||
		sender.id != jobs.job.ID {
		t.Fatal("incorrect email delivery")
	}
}

func TestEmailWorkerRetry(t *testing.T) {
	worker, jobs, sender := newEmailWorkerFixture(1)
	sender.err = errors.New("temporary SMTP failure")

	processed, err := worker.ProcessOne(
		context.Background(),
	)
	if err != nil {
		t.Fatal(err)
	}

	if !processed || jobs.finalState != "pending" {
		t.Fatal("expected job to be retried")
	}

	if !jobs.nextRetry.After(time.Now()) {
		t.Fatal("retry must be scheduled in the future")
	}
}

func TestEmailWorkerMaxAttempts(t *testing.T) {
	worker, jobs, sender := newEmailWorkerFixture(5)
	sender.err = errors.New("delivery failed")

	_, err := worker.ProcessOne(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if jobs.finalState != "dead" {
		t.Fatal("expected dead job after max attempts")
	}
}

func TestEmailWorkerSkipsInvalidVerification(t *testing.T) {
	worker, jobs, sender := newEmailWorkerFixture(1)
	jobs.allowed = false

	_, err := worker.ProcessOne(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if jobs.finalState != "dead" {
		t.Fatal("invalid verification must be discarded")
	}

	if sender.calls != 0 {
		t.Fatal("invalid verification must not be sent")
	}
}

func TestEmailWorkerNoJobs(t *testing.T) {
	worker, jobs, sender := newEmailWorkerFixture(1)
	jobs.claimed = true

	processed, err := worker.ProcessOne(
		context.Background(),
	)
	if err != nil {
		t.Fatal(err)
	}

	if processed || sender.calls != 0 {
		t.Fatal("expected no work")
	}
}
