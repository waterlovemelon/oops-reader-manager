package catalog

import "time"

type BookStatus string

const (
	StatusDraft   BookStatus = "draft"
	StatusActive  BookStatus = "active"
	StatusHidden  BookStatus = "hidden"
	StatusDeleted BookStatus = "deleted"
)

type Book struct {
	BookKey          string
	Title            string
	Author           string
	Description      string
	Format           string
	Filename         string
	StoragePath      string
	CoverStoragePath string
	FileSize         int64
	ContentSHA1      string
	Language         string
	ChapterCount     int
	Status           BookStatus
	Source           string
	UploadedAt       *time.Time
	PublishedAt      *time.Time
	DeletedAt        *time.Time
	UpdatedBy        string
}
