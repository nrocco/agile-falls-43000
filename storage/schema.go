package storage

import (
	"context"
)

const schema = `
PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS bookmarks (
	id CHAR(16) PRIMARY KEY,
	created DATE DEFAULT (datetime('now')),
	updated DATE DEFAULT (datetime('now')),
	title VARCHAR(64) NOT NULL,
	url VARCHAR(255) UNIQUE NOT NULL,
	excerpt TEXT NOT NULL DEFAULT '',
	content TEXT NOT NULL DEFAULT '',
	tags JSON NOT NULL DEFAULT '[]',
	archived BOOLEAN NOT NULL DEFAULT 0
);

CREATE VIRTUAL TABLE IF NOT EXISTS bookmarks_fts
USING fts5(title, url, content, content=bookmarks, content_rowid=rowid);

CREATE TRIGGER IF NOT EXISTS bookmarks_ai AFTER INSERT ON bookmarks BEGIN
	INSERT INTO bookmarks_fts(rowid, title, url, content) VALUES (new.rowid, new.title, new.url, new.content);
END;

CREATE TRIGGER IF NOT EXISTS bookmarks_ad AFTER DELETE ON bookmarks BEGIN
	INSERT INTO bookmarks_fts(bookmarks_fts, rowid, title, url, content) VALUES('delete', old.rowid, old.title, old.url, old.content);
END;

CREATE TRIGGER IF NOT EXISTS bookmarks_au AFTER UPDATE ON bookmarks BEGIN
	INSERT INTO bookmarks_fts(bookmarks_fts, rowid, title, url, content) VALUES('delete', old.rowid, old.title, old.url, old.content);
	INSERT INTO bookmarks_fts(rowid, title, url, content) VALUES (new.rowid, new.title, new.url, new.content);
END;

CREATE TABLE IF NOT EXISTS feeds (
	id CHAR(16) PRIMARY KEY,
	created DATE DEFAULT (datetime('now')),
	updated DATE DEFAULT (datetime('now')),
	refreshed DATE DEFAULT (datetime('now')),
	last_authored DATE DEFAULT (datetime('now')),
	title VARCHAR(64) NOT NULL,
	url VARCHAR(255) UNIQUE NOT NULL,
	etag VARCHAR(200) NOT NULL DEFAULT '',
	tags JSON NOT NULL DEFAULT '[]',
	items JSON NOT NULL DEFAULT '[]'
);

CREATE TABLE IF NOT EXISTS users (
	id CHAR(16) PRIMARY KEY,
	created DATE NOT NULL,
	updated DATE NOT NULL,
	username VARCHAR(64) NOT NULL UNIQUE,
	password VARCHAR(255) NOT NULL,
	token VARCHAR(255) NOT NULL UNIQUE
) WITHOUT ROWID;

CREATE TABLE IF NOT EXISTS thoughts (
	id CHAR(16) PRIMARY KEY,
	created DATE NOT NULL,
	updated DATE NOT NULL,
	title VARCHAR(255) UNIQUE NOT NULL,
	tags JSON NOT NULL DEFAULT '[]',
	content TEXT NOT NULL DEFAULT ''
);

CREATE VIRTUAL TABLE IF NOT EXISTS thoughts_fts
USING fts5(title, content, content=thoughts, content_rowid=rowid);

CREATE TRIGGER IF NOT EXISTS thoughts_ai AFTER INSERT ON thoughts BEGIN
	INSERT INTO thoughts_fts(rowid, title, content) VALUES (new.rowid, new.title, new.content);
END;

CREATE TRIGGER IF NOT EXISTS thoughts_ad AFTER DELETE ON thoughts BEGIN
	INSERT INTO thoughts_fts(thoughts_fts, rowid, title, content) VALUES('delete', old.rowid, old.title, old.content);
END;

CREATE TRIGGER IF NOT EXISTS thoughts_au AFTER UPDATE ON thoughts BEGIN
	INSERT INTO thoughts_fts(thoughts_fts, rowid, title, content) VALUES('delete', old.rowid, old.title, old.content);
	INSERT INTO thoughts_fts(rowid, title, content) VALUES (new.rowid, new.title, new.content);
END;
`

func (store *Store) migrate(ctx context.Context) error {
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, schema); err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return err
}
