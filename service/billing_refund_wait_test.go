package service

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestWaitPendingBillingRefundsWaitsForInFlightWork(t *testing.T) {
	var started atomic.Bool
	var finished atomic.Bool

	pendingBillingRefunds.Add(1)
	go func() {
		defer pendingBillingRefunds.Done()
		started.Store(true)
		time.Sleep(50 * time.Millisecond)
		finished.Store(true)
	}()

	WaitPendingBillingRefunds(time.Second)
	assert.True(t, started.Load())
	assert.True(t, finished.Load())
}

func TestWaitPendingBillingRefundsRespectsTimeout(t *testing.T) {
	pendingBillingRefunds.Add(1)
	released := make(chan struct{})
	go func() {
		defer pendingBillingRefunds.Done()
		<-released
	}()

	start := time.Now()
	WaitPendingBillingRefunds(30 * time.Millisecond)
	assert.Less(t, time.Since(start), 300*time.Millisecond)
	close(released)
	WaitPendingBillingRefunds(time.Second)
}
