package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

var testIntervals []time.Duration = []time.Duration{time.Duration(time.Second * 10)}

func TestMessageShouldBeSent(t *testing.T) {
	created := time.Now()
	msg := scheduledMessage{0, 0, created, created, false}

	time.Sleep(time.Second / 10)
	require.True(t, msg.shouldBeSent(), "Message should be sent already")

	msg.updateNextAt(testIntervals)
	require.False(t, msg.shouldBeSent(), "Message should be sent later")
}

func TestSliceOfMessages(t *testing.T) {
	created := time.Now()
	msg := scheduledMessage{0, 0, created, created, false}

	var slice []scheduledMessage
	slice = append(slice, msg)
	time.Sleep(time.Second / 10)

	msgFromSlice := &slice[0]
	msgFromSlice.updateNextAt(testIntervals)
	require.False(t, msgFromSlice.shouldBeSent(), "Magically message from slice was not updated")
	require.False(t, slice[0].shouldBeSent(), "Source message has not been updated")
}

// TODO add tests for handleUpdate and scheduleReminders
