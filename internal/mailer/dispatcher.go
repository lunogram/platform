package mailer

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
)

// dispatchQueueSize bounds how much mail may be waiting for a worker. Auth mail
// arrives at human pace, so a queue this deep is only ever occupied while an
// SMTP server is slow or down; once it fills, dropping is the correct answer
// because the alternative is stalling the requests that produce it.
const dispatchQueueSize = 256

// dispatchWorkers is how many deliveries may be in flight at once. SMTP
// delivery is dominated by network round trips, and a handful of connections is
// plenty for the volume auth mail produces while staying inside the connection
// limits shared relays impose.
const dispatchWorkers = 4

// Dispatcher sends mail off the request goroutine.
//
// Every auth send goes through it, because none of them may block or fail the
// HTTP request that triggered it: registration, reset and change-password all
// answer identically whether or not the mail was delivered, and a mail failure
// that surfaced to the caller would itself be an account-enumeration oracle.
// Failures are logged and go no further.
type Dispatcher struct {
	mailer  Mailer
	logger  *zap.Logger
	timeout time.Duration

	queue chan Message

	stop sync.Once
	done chan struct{}
	wg   sync.WaitGroup
}

func NewDispatcher(mailer Mailer, logger *zap.Logger, timeout time.Duration) *Dispatcher {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	d := &Dispatcher{
		mailer:  mailer,
		logger:  logger,
		timeout: timeout,
		queue:   make(chan Message, dispatchQueueSize),
		done:    make(chan struct{}),
	}

	d.wg.Add(dispatchWorkers)
	for range dispatchWorkers {
		go d.work()
	}

	return d
}

// Dispatch queues a message for delivery. It never blocks and never reports
// failure: the caller is an HTTP handler whose response must not depend on what
// happens to the mail.
func (d *Dispatcher) Dispatch(message Message) {
	if d == nil {
		return
	}

	select {
	case d.queue <- message:
	default:
		// The recipient is PII and the action URL is a bearer credential, so the
		// drop is recorded with neither -- and not with the subject either,
		// which is an operator-supplied template that may interpolate both. The
		// kind is a fixed identifier and says as much about which flow is
		// affected. A full queue means the transport is unhealthy, which is the
		// thing worth alerting on.
		d.logger.Error("dropped an outgoing message: the mail queue is full",
			zap.String("kind", message.Kind))
	}
}

func (d *Dispatcher) work() {
	defer d.wg.Done()

	for {
		select {
		case message := <-d.queue:
			d.send(message)
		case <-d.done:
			// Drain what is already queued so a shutdown does not swallow a
			// verification link somebody is waiting for.
			for {
				select {
				case message := <-d.queue:
					d.send(message)
				default:
					return
				}
			}
		}
	}
}

func (d *Dispatcher) send(message Message) {
	// The request context is long gone (and would have been cancelled the moment
	// the response was written), so delivery runs on its own bounded one.
	ctx, cancel := context.WithTimeout(context.Background(), d.timeout)
	defer cancel()

	if err := d.mailer.Send(ctx, message); err != nil {
		d.logger.Error("failed to deliver a message",
			zap.String("kind", message.Kind),
			zap.Error(err))
	}
}

// Close stops the workers once the queue has drained.
func (d *Dispatcher) Close() {
	if d == nil {
		return
	}
	d.stop.Do(func() { close(d.done) })
	d.wg.Wait()
}
