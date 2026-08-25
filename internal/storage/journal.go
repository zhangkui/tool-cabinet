package storage

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"os"
	"sync"
	"time"
)

type JournalEntry struct {
	Sequence uint64            `json:"sequence"`
	At       time.Time         `json:"at"`
	Actor    string            `json:"actor"`
	Action   string            `json:"action"`
	Entity   string            `json:"entity"`
	EntityID string            `json:"entity_id"`
	Payload  map[string]string `json:"payload,omitempty"`
}
type Journal struct {
	mu       sync.Mutex
	file     *os.File
	sequence uint64
}

func OpenJournal(path string) (*Journal, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}
	journal := &Journal{file: file}
	if err := journal.scan(); err != nil {
		_ = file.Close()
		return nil, err
	}
	return journal, nil
}
func (j *Journal) Append(entry JournalEntry) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.sequence++
	entry.Sequence = j.sequence
	if entry.At.IsZero() {
		entry.At = time.Now()
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	if _, err := j.file.Write(append(data, '\n')); err != nil {
		return err
	}
	return j.file.Sync()
}
func (j *Journal) scan() error {
	if _, err := j.file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	scanner := bufio.NewScanner(j.file)
	for scanner.Scan() {
		var entry JournalEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			return err
		}
		if entry.Sequence > j.sequence {
			j.sequence = entry.Sequence
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	_, err := j.file.Seek(0, io.SeekEnd)
	return err
}
func (j *Journal) Since(sequence uint64) ([]JournalEntry, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if _, err := j.file.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	scanner := bufio.NewScanner(j.file)
	result := make([]JournalEntry, 0)
	for scanner.Scan() {
		var entry JournalEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			return nil, err
		}
		if entry.Sequence > sequence {
			result = append(result, entry)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	_, err := j.file.Seek(0, io.SeekEnd)
	return result, err
}
func (j *Journal) Close() error {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.file == nil {
		return errors.New("journal already closed")
	}
	err := j.file.Close()
	j.file = nil
	return err
}
