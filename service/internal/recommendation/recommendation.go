package recommendation

import "time"

type Status string

const (
	StatusActive  Status = "active"
	StatusDeleted Status = "deleted"
)

type PublishState string

const (
	PublishStateQueued    PublishState = "queued"
	PublishStatePublished PublishState = "published"
	PublishStateDeleted   PublishState = "deleted"
)

type Recommendation struct {
	ID                int64
	BookKey           string
	Comment           string
	Status            Status
	ScheduledPublishAt time.Time
	CreatedBy         string
	UpdatedBy         string
	DeletedAt         *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type BookSnapshot struct {
	BookKey     string
	Title       string
	Author      string
	Description string
	CoverPath   string
}

func (r Recommendation) PublishState(now time.Time) PublishState {
	if r.Status == StatusDeleted {
		return PublishStateDeleted
	}
	if !r.ScheduledPublishAt.After(now) {
		return PublishStatePublished
	}
	return PublishStateQueued
}
