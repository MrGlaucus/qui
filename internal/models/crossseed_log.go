package models

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/autobrr/qui/internal/dbinterface"
)

// CrossSeedLogEntry records a cross-seed event for a torrent infohash.
// PublishDate is only set when the Torznab/RSS source provided it.
// The entry is created on successful cross-seed and used for cross-instance dedup.
type CrossSeedLogEntry struct {
	InfoHash      string    `json:"infoHash"`
	InstanceID    int       `json:"instanceId"`
	TorrentName   string    `json:"torrentName"`
	SourceIndexer string    `json:"sourceIndexer"`
	PublishDate   time.Time `json:"publishDate"`
	CreatedAt     time.Time `json:"createdAt"`
}

type CrossSeedLogStore struct {
	db dbinterface.Querier
}

func NewCrossSeedLogStore(db dbinterface.Querier) *CrossSeedLogStore {
	return &CrossSeedLogStore{db: db}
}

func (s *CrossSeedLogStore) Upsert(ctx context.Context, infohash string, instanceID int, torrentName string, sourceIndexer string, publishDate *time.Time) error {
	var pubDateVal any
	if publishDate != nil {
		pubDateVal = publishDate.UTC().Format("2006-01-02 15:04:05")
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO cross_seed_log (infohash, instance_id, torrent_name, source_indexer, publish_date)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(infohash)
		DO UPDATE SET instance_id = excluded.instance_id, torrent_name = excluded.torrent_name, source_indexer = excluded.source_indexer, publish_date = COALESCE(excluded.publish_date, cross_seed_log.publish_date)
	`, normalizeInfoHash(infohash), instanceID, torrentName, sourceIndexer, pubDateVal)
	return err
}

// scanPublishDate parses a publish_date string from SQLite into time.Time.
// SQLite stores TIMESTAMP as TEXT; Go's time.Time Scan doesn't handle all formats.
func scanPublishDate(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	// Try formats used by go-sqlite3 driver and SQLite native CURRENT_TIMESTAMP
	formats := []string{
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05.999999999Z07:00",
		"2006-01-02 15:04:05",
		time.RFC3339Nano,
		time.RFC3339,
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

func (s *CrossSeedLogStore) Get(ctx context.Context, infohash string) (*CrossSeedLogEntry, bool, error) {
	var entry CrossSeedLogEntry
	var pubDateStr sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT infohash, instance_id, torrent_name, source_indexer, publish_date, created_at
		FROM cross_seed_log
		WHERE infohash = ?
	`, normalizeInfoHash(infohash)).Scan(&entry.InfoHash, &entry.InstanceID, &entry.TorrentName, &entry.SourceIndexer, &pubDateStr, &entry.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		}
		return nil, false, err
	}
	if pubDateStr.Valid {
		entry.PublishDate = scanPublishDate(pubDateStr.String)
	}
	return &entry, true, nil
}

func (s *CrossSeedLogStore) List(ctx context.Context, limit, offset int) ([]*CrossSeedLogEntry, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT infohash, instance_id, torrent_name, source_indexer, publish_date, created_at
		FROM cross_seed_log
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []*CrossSeedLogEntry
	for rows.Next() {
		var entry CrossSeedLogEntry
		var pubDateStr sql.NullString
		if err := rows.Scan(&entry.InfoHash, &entry.InstanceID, &entry.TorrentName, &entry.SourceIndexer, &pubDateStr, &entry.CreatedAt); err != nil {
			return nil, err
		}
		if pubDateStr.Valid {
			entry.PublishDate = scanPublishDate(pubDateStr.String)
		}
		entries = append(entries, &entry)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}

func (s *CrossSeedLogStore) Count(ctx context.Context) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM cross_seed_log`).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// DeleteOlderThan removes entries created before the given time.
func (s *CrossSeedLogStore) DeleteOlderThan(ctx context.Context, before time.Time) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM cross_seed_log WHERE created_at < ?`, before.UTC())
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// FindByHashes returns the first matching infohash from the provided list.
// Used for cross-instance dedup: if any of the given hashes already have a log
// entry (from any instance), the cross-seed should be skipped.
func (s *CrossSeedLogStore) FindByHashes(ctx context.Context, hashes []string) (string, bool, error) {
	if len(hashes) == 0 {
		return "", false, nil
	}
	normalized := normalizeInfoHashList(hashes)
	if len(normalized) == 0 {
		return "", false, nil
	}
	placeholders := buildPlaceholders(len(normalized))
	query := fmt.Sprintf(`
		SELECT infohash
		FROM cross_seed_log
		WHERE infohash IN (%s)
		LIMIT 1
	`, placeholders)
	args := make([]any, 0, len(normalized))
	for _, h := range normalized {
		args = append(args, h)
	}
	var infohash string
	if err := s.db.QueryRowContext(ctx, query, args...).Scan(&infohash); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", false, nil
		}
		return "", false, err
	}
	return infohash, true, nil
}


