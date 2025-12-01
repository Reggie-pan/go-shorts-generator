package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/Reggie-pan/go-shorts-generator/internal/service/job"
)

type Store struct {
	path string
	mu   sync.RWMutex
	data map[string]*job.Record
}

func NewStore(base string) (*Store, error) {
	if err := os.MkdirAll(base, 0o755); err != nil {
		return nil, err
	}
	dbPath := filepath.Join(base, "jobs.json")
	s := &Store{path: dbPath, data: map[string]*job.Record{}}
	if b, err := os.ReadFile(dbPath); err == nil {
		_ = json.Unmarshal(b, &s.data)
	}
	return s, nil
}

func (s *Store) Close() error {
	return nil
}

func (s *Store) persist() error {
	tmp := filepath.Join(filepath.Dir(s.path), "jobs.tmp")
	b, _ := json.MarshalIndent(s.data, "", "  ")
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func (s *Store) InsertJob(r *job.Record) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[r.ID] = r
	return s.persist()
}

func (s *Store) UpdateJob(r *job.Record) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[r.ID] = r
	return s.persist()
}

func (s *Store) GetJob(id string) (*job.Record, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.data[id]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	return r, nil
}

func (s *Store) ListJobs(page, limit int) ([]*job.Record, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	values := []*job.Record{}
	for _, v := range s.data {
		values = append(values, v)
	}
	total := len(values)
	start := (page - 1) * limit
	if start >= total {
		return []*job.Record{}, total, nil
	}
	end := start + limit
	if end > total {
		end = total
	}
	return values[start:end], total, nil
}

func (s *Store) DeleteJob(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, id)
	return s.persist()
}

func (s *Store) DeleteAllJobs() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data = make(map[string]*job.Record)
	return s.persist()
}
