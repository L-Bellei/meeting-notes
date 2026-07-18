//go:build windows

package main

import "testing"

func chanClosed(ch chan struct{}) bool {
	select {
	case <-ch:
		return true
	default:
		return false
	}
}

// A late terminal-status event (transcribing/processing/completed/failed) from a
// PREVIOUS meeting must not hide the overlay that a NEWER recording is showing.
func TestOverlay_HideIfMeeting_IgnoresStaleMeeting(t *testing.T) {
	o := &OverlayWindow{}
	stopCh := make(chan struct{})
	o.meetingID = "meeting-B"
	o.stopCh = stopCh

	o.HideIfMeeting("meeting-A")

	if chanClosed(stopCh) {
		t.Fatal("stale meeting stopped the active overlay timer")
	}
	if o.stopCh == nil {
		t.Fatal("stale meeting cleared the active overlay state")
	}
}

// A terminal-status event for the meeting currently shown must hide it.
func TestOverlay_HideIfMeeting_HidesCurrentMeeting(t *testing.T) {
	o := &OverlayWindow{}
	stopCh := make(chan struct{})
	o.meetingID = "meeting-B"
	o.stopCh = stopCh

	o.HideIfMeeting("meeting-B")

	if !chanClosed(stopCh) {
		t.Fatal("current meeting did not stop the overlay timer")
	}
}
