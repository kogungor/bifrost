package snapshot

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kogungor/bifrost/internal/security"
)

type TimelineEvent struct {
	Timestamp time.Time `json:"ts"`
	Type      string    `json:"type"`
	Snapshot  string    `json:"snapshot,omitempty"`
	Plan      string    `json:"plan,omitempty"`
	Step      string    `json:"step,omitempty"`
	Check     string    `json:"check,omitempty"`
	Status    string    `json:"status,omitempty"`
	Task      string    `json:"task,omitempty"`
}

func TimelinePath(projectRoot string) string {
	return filepath.Join(Dir(projectRoot), "timeline.jsonl")
}

func AppendTimelineEvent(projectRoot string, event TimelineEvent) error {
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	} else {
		event.Timestamp = event.Timestamp.UTC()
	}
	event = sanitizeTimelineEvent(projectRoot, event)
	if err := os.MkdirAll(Dir(projectRoot), 0700); err != nil {
		return err
	}
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(TimelinePath(projectRoot), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(append(data, '\n')); err != nil {
		return err
	}
	return nil
}

func ReadTimeline(projectRoot string, limit int) ([]TimelineEvent, error) {
	f, err := os.Open(TimelinePath(projectRoot))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var events []TimelineEvent
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var event TimelineEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}
		events = append(events, event)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if limit > 0 && len(events) > limit {
		events = events[len(events)-limit:]
	}
	for i, j := 0, len(events)-1; i < j; i, j = i+1, j-1 {
		events[i], events[j] = events[j], events[i]
	}
	return events, nil
}

func sanitizeTimelineEvent(projectRoot string, event TimelineEvent) TimelineEvent {
	cfg := security.LoadConfig(projectRoot)
	event.Type = redactTimelineString(event.Type, cfg)
	event.Snapshot = redactTimelineString(event.Snapshot, cfg)
	event.Plan = redactTimelineString(event.Plan, cfg)
	event.Step = redactTimelineString(event.Step, cfg)
	event.Check = redactTimelineString(event.Check, cfg)
	event.Status = redactTimelineString(event.Status, cfg)
	event.Task = redactTimelineString(event.Task, cfg)
	return event
}

func redactTimelineString(value string, cfg security.Config) string {
	if value == "" {
		return ""
	}
	redacted, _ := security.RedactString(value, cfg)
	return redacted
}
