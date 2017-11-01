package storage

import (
	"errors"
	"time"

	sqlite3 "github.com/mattn/go-sqlite3"
	"github.com/mmcdole/gofeed"
	log "github.com/sirupsen/logrus"
)

type Feed struct {
	ID        int64
	Created   time.Time
	Updated   time.Time
	Refreshed time.Time
	Title     string
	URL       string
}

// Validate is used to assert Title and URL are set
func (feed *Feed) Validate() error {
	if feed.URL == "" {
		return errors.New("Missing Feed.URL")
	}

	return nil
}

type ListFeedsOptions struct {
	Search            string
	NotRefreshedSince time.Time
	Limit             int
	Offset            int
}

// ListFeeds fetches multiple feeds from the database
func (store *Store) ListFeeds(options *ListFeedsOptions) (*[]*Feed, int) {
	query := store.db.Select("feeds")

	if options.Search != "" {
		query.Where("(title LIKE ? OR url LIKE ?)", "%"+options.Search+"%", "%"+options.Search+"%")
	}

	if !options.NotRefreshedSince.IsZero() {
		query.Where("refreshed < ?", options.NotRefreshedSince)
	}

	feeds := []*Feed{}
	totalCount := 0

	query.Columns("COUNT(id)")
	query.LoadValue(&totalCount)

	query.Columns("*")
	query.OrderBy("refreshed", "DESC")
	query.Limit(options.Limit)
	query.Offset(options.Offset)
	query.Load(&feeds)

	return &feeds, totalCount
}

// GetFeed finds a single feed by ID or URL
func (store *Store) GetFeed(feed *Feed) error {
	query := store.db.Select("feeds")
	query.Limit(1)

	if feed.ID != 0 {
		query.Where("id = ?", feed.ID)
	} else if feed.URL != "" {
		query.Where("url = ?", feed.URL)
	} else {
		return errors.New("Missing Feed.ID or Feed.URL")
	}

	if err := query.LoadValue(&feed); err != nil {
		return err
	}

	return nil
}

// AddFeed persists a feed to the database and schedules an async job to fetch the content
func (store *Store) AddFeed(feed *Feed) error {
	if feed.ID != 0 {
		return errors.New("Existing feed")
	}

	if err := feed.Validate(); err != nil {
		return err
	}

	feed.Created = time.Now()
	feed.Updated = time.Now()
	feed.Refreshed = time.Time{}

	query := store.db.Insert("feeds")
	query.Columns("created", "updated", "refreshed", "title", "url")
	query.Record(feed)

	l := log.WithFields(log.Fields{
		"id":    feed.ID,
		"title": feed.Title,
		"url":   feed.URL,
	})

	if _, err := query.Exec(); err != nil {
		if exists := err.(sqlite3.Error).ExtendedCode == sqlite3.ErrConstraintUnique; exists {
			// TODO get the existing feed from the database to fill the Feed.ID field properly
			l.Info("Feed already exists")
			return nil
		}

		l.WithError(err).Error("Error persisting feed")
		return err
	}

	l.Info("Persisted feed")

	// TODO move this: WorkQueue <- WorkRequest{Type: "Feed.Refresh", Feed: *feed}

	return nil
}

// UpdateFeed updates the given feed
func (store *Store) UpdateFeed(feed *Feed) error {
	if feed.ID == 0 {
		return errors.New("Not an existing feed")
	}

	if err := feed.Validate(); err != nil {
		return err
	}

	feed.Updated = time.Now()

	query := store.db.Update("feeds")
	query.Set("updated", feed.Updated)
	query.Set("refreshed", feed.Refreshed)
	query.Set("title", feed.Title)
	query.Set("url", feed.URL)
	query.Where("id = ?", feed.ID)

	if _, err := query.Exec(); err != nil {
		return err
	}

	return nil
}

// DeleteFeed deletes the given feed from the database
func (store *Store) DeleteFeed(feed *Feed) error {
	if feed.ID == 0 {
		return errors.New("Not an existing feed")
	}

	query := store.db.Delete("items")
	query.Where("feed_id = ?", feed.ID)

	if _, err := query.Exec(); err != nil {
		return err
	}

	query = store.db.Delete("feeds")
	query.Where("id = ?", feed.ID)

	if _, err := query.Exec(); err != nil {
		return err
	}

	return nil
}

// RefreshFeed fetches the rss feed items and persists those to the database
func (store *Store) RefreshFeed(feed *Feed) error {
	if feed.ID == 0 {
		return errors.New("Not an existing feed")
	}

	fp := gofeed.NewParser()

	parsedFeed, err := fp.ParseURL(feed.URL)
	if err != nil {
		return err
	}

	for _, item := range parsedFeed.Items {
		date := item.PublishedParsed
		if date == nil {
			date = item.UpdatedParsed
		}

		if date.Before(feed.Refreshed) {
			log.Printf("Ignoring '%s' since we already fetched it before", item.Title)
			continue
		}

		content := item.Content
		if content == "" {
			content = item.Description
		}

		query := store.db.Insert("items")
		query.Columns("feed_id", "created", "updated", "title", "url", "date", "content")
		query.Values(feed.ID, time.Now(), time.Now(), item.Title, item.Link, date, content)

		if _, err := query.Exec(); err != nil {
			log.Println(err)
		}
	}

	feed.Title = parsedFeed.Title
	feed.Refreshed = time.Now()

	if err := store.UpdateFeed(feed); err != nil {
		return err
	}

	return nil
}