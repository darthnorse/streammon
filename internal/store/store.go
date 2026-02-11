package store

import (
	"database/sql"

	_ "modernc.org/sqlite"

	"streammon/internal/crypto"
)

type Store struct {
	db        *sql.DB
	encryptor *crypto.Encryptor
}

type Option func(*Store)

func WithEncryptor(e *crypto.Encryptor) Option {
	return func(s *Store) { s.encryptor = e }
}

func New(dbPath string, opts ...Option) (*Store, error) {
	db, err := sql.Open("sqlite", "file:"+dbPath+"?_pragma=journal_mode(wal)&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_time_format=sqlite")
	if err != nil {
		return nil, err
	}
	if dbPath == ":memory:" {
		db.SetMaxOpenConns(1)
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}
	s := &Store{db: db}
	for _, o := range opts {
		o(s)
	}
	return s, nil
}

// HasEncryptor reports whether the store was initialized with an encryption key.
func (s *Store) HasEncryptor() bool {
	return s.encryptor != nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) Ping() error {
	return s.db.Ping()
}
