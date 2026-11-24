// Package notices keeps the messages the window has reported to the user: what
// it said, when, and whether the user has seen it yet. They survive a restart
// in a file of their own beside the configuration, so that the history the
// window shows is what the user left there. It reads and writes JSON alone,
// with no UI, so it tests without a window.
package notices

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

// Version is the schema version of the file Save writes.
const Version = 1

// FileName is the name of the file Load reads and Save writes.
const FileName = "notices.json"

// Message is one message the window reported.
type Message struct {
	// ID numbers the messages of a user in the order they were reported, one
	// above the last: it names a message for removal, and tells the window
	// which of them it has toasted already. It is never reused, not even
	// after the history is cleared.
	ID int `json:"id"`
	// Kind is what the message reports, as "error" or "success", for the look
	// to tell.
	Kind string `json:"kind"`
	// Text is what it says.
	Text string `json:"text"`
	// At is when the window reported it.
	At time.Time `json:"at"`
	// Seen is whether the user has opened the history since.
	Seen bool `json:"seen"`
}

// file is what Save writes and Load reads.
type file struct {
	Version  int       `json:"version"`
	Messages []Message `json:"messages"`
}

// Store is the messages of one user, the newest first. There is no limit: the
// history keeps every message until the user removes one or clears them.
type Store struct {
	// path is where it saves, "" while it keeps them in memory alone.
	path string
	msgs []Message
	next int
}

// Load reads the messages in dir. A directory without a file gives an empty
// store rather than an error, as does an empty dir, which keeps the store in
// memory and saves nothing. A file that cannot be read is reported, and leaves
// an empty store rather than half of its messages: writing it back must not
// spread the damage.
func Load(dir string) (*Store, error) {
	s := &Store{next: 1}
	if dir == "" {
		return s, nil
	}
	s.path = filepath.Join(dir, FileName)
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return s, err
	}
	var f file
	if err := json.Unmarshal(data, &f); err != nil {
		return s, err
	}
	for _, m := range f.Messages {
		if m.Text == "" {
			continue
		}
		s.msgs = append(s.msgs, m)
		s.next = max(s.next, m.ID+1)
	}
	return s, nil
}

// Messages returns the messages, the newest first.
func (s *Store) Messages() []Message { return s.msgs }

// Unseen is how many messages the user has not seen.
func (s *Store) Unseen() int {
	n := 0
	for _, m := range s.msgs {
		if !m.Seen {
			n++
		}
	}
	return n
}

// Last is the ID of the newest message, 0 without any.
func (s *Store) Last() int {
	if len(s.msgs) == 0 {
		return 0
	}
	return s.msgs[0].ID
}

// Add records a message, and saves the history. An empty text is not a
// message, and is dropped.
func (s *Store) Add(kind, text string, at time.Time) (Message, error) {
	if text == "" {
		return Message{}, nil
	}
	m := Message{ID: s.next, Kind: kind, Text: text, At: at}
	s.next++
	s.msgs = append([]Message{m}, s.msgs...)
	return m, s.Save()
}

// Remove forgets the message with id, and saves the history. An id no message
// has is ignored.
func (s *Store) Remove(id int) error {
	kept := s.msgs[:0]
	found := false
	for _, m := range s.msgs {
		if m.ID == id {
			found = true
			continue
		}
		kept = append(kept, m)
	}
	s.msgs = kept
	if !found {
		return nil
	}
	return s.Save()
}

// Clear forgets every message, and saves the history. The IDs of the messages
// that come after go on above the ones cleared.
func (s *Store) Clear() error {
	if len(s.msgs) == 0 {
		return nil
	}
	s.msgs = nil
	return s.Save()
}

// MarkSeen marks every message seen, as opening the history does, and saves
// the history when that changed anything.
func (s *Store) MarkSeen() error {
	changed := false
	for i := range s.msgs {
		if s.msgs[i].Seen {
			continue
		}
		s.msgs[i].Seen = true
		changed = true
	}
	if !changed {
		return nil
	}
	return s.Save()
}

// Save writes the history where Load reads it, through a temporary file so
// that a write cut short leaves the last one whole. A store without a
// directory saves nothing.
func (s *Store) Save() error {
	if s.path == "" {
		return nil
	}
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	msgs := s.msgs
	if msgs == nil {
		msgs = []Message{}
	}
	data, err := json.MarshalIndent(file{Version: Version, Messages: msgs}, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp, err := os.CreateTemp(dir, FileName+".*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, s.path)
}
