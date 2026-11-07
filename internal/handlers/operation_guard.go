package handlers

import (
	"errors"
	"net/http"

	"meeting-notes/internal/services"
)

func runMeetingIdle[T any](operations *services.MeetingOperationCoordinator, meetingID string, fn func() (T, error)) (T, error) {
	var zero T
	if operations == nil {
		return fn()
	}
	var result T
	err := operations.WithMeetingLock(meetingID, func(operation string) error {
		if operation != "" {
			return services.ErrMeetingBusy
		}
		var err error
		result, err = fn()
		return err
	})
	if err != nil {
		return zero, err
	}
	return result, nil
}

func runMeetingIdleError(operations *services.MeetingOperationCoordinator, meetingID string, fn func() error) error {
	_, err := runMeetingIdle(operations, meetingID, func() (struct{}, error) {
		return struct{}{}, fn()
	})
	return err
}

func writeMeetingBusy(w http.ResponseWriter, err error) bool {
	if !errors.Is(err, services.ErrMeetingBusy) {
		return false
	}
	writeError(w, http.StatusConflict, "meeting already has an active operation")
	return true
}
