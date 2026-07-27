package self_check

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAttemptFixTaskRejectsUnfixableTask(t *testing.T) {
	err := attemptFixTask(&Task{})
	assert.ErrorIs(t, err, ErrTaskNotFixable)
}

func TestAttemptFixTaskReturnsFixError(t *testing.T) {
	wantErr := errors.New("fix failed")
	task := &Task{
		FixFunc: func() error { return wantErr },
		CheckFunc: func() error {
			t.Fatal("check must not run after a failed fix")
			return nil
		},
	}

	err := attemptFixTask(task)
	assert.ErrorIs(t, err, wantErr)
}

func TestAttemptFixTaskVerifiesPostcondition(t *testing.T) {
	wantErr := errors.New("still broken")
	task := &Task{
		FixFunc:   func() error { return nil },
		CheckFunc: func() error { return wantErr },
	}

	err := attemptFixTask(task)
	assert.ErrorIs(t, err, wantErr)
}

func TestAttemptFixTaskSucceedsAfterVerification(t *testing.T) {
	fixCalls := 0
	checkCalls := 0
	task := &Task{
		FixFunc: func() error {
			fixCalls++
			return nil
		},
		CheckFunc: func() error {
			checkCalls++
			return nil
		},
	}

	require.NoError(t, attemptFixTask(task))
	assert.Equal(t, 1, fixCalls)
	assert.Equal(t, 1, checkCalls)
}
