package services

import (
	"errors"
	"sync"
)

var ErrMeetingBusy = errors.New("meeting already has an active operation")

type MeetingOperationCoordinator struct {
	mu     sync.Mutex
	active map[string]string
	closed bool
}

func NewMeetingOperationCoordinator() *MeetingOperationCoordinator {
	return &MeetingOperationCoordinator{active: make(map[string]string)}
}

func (c *MeetingOperationCoordinator) Begin(meetingID, operation string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return ErrMeetingBusy
	}
	if _, ok := c.active[meetingID]; ok {
		return ErrMeetingBusy
	}
	c.active[meetingID] = operation
	return nil
}

func (c *MeetingOperationCoordinator) BeginStopRecording(meetingID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return ErrMeetingBusy
	}
	if operation, ok := c.active[meetingID]; ok {
		if operation != "recording" {
			return ErrMeetingBusy
		}
	}
	c.active[meetingID] = "pipeline"
	return nil
}

func (c *MeetingOperationCoordinator) End(meetingID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.active, meetingID)
}

func (c *MeetingOperationCoordinator) WithMeetingLock(meetingID string, fn func(operation string) error) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return fn(c.active[meetingID])
}

func (c *MeetingOperationCoordinator) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
}
