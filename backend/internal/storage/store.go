package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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
		// 補齊重啟載入時丟失的 BasePath 記憶體狀態 (B2)
		for id, rec := range s.data {
			if rec.BasePath == "" {
				rec.BasePath = filepath.Join(base, "jobs", id)
			}
		}
	}
	return s, nil
}

func (s *Store) Close() error {
	return nil
}

func (s *Store) persist() error {
	tmp := filepath.Join(filepath.Dir(s.path), "jobs.tmp")
	b, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal job store: %w", err)
	}
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

// ListJobs 獲取任務列表
// ponytail: 當 Job 數量小於 10,000 時，全排序耗時均在 5ms 內，故採用簡單的 RWMutex 保證執行期安全性。
// 若未來 Job 積累量達數萬筆，升級路徑為在寫入時維護已排序的 slice cache，或引入輕量級 SQLite 持久化索引。
func (s *Store) ListJobs(page, limit int) ([]*job.Record, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	values := []*job.Record{}
	for _, v := range s.data {
		values = append(values, v)
	}

	// 按建立時間降序排列（最新的在最前面）
	sort.Slice(values, func(i, j int) bool {
		return values[i].CreatedAt.After(values[j].CreatedAt)
	})

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
