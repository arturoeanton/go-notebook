package main

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/arturoeanton/go-echo-live-view/liveview"
	"github.com/cosmos72/gomacro/fast"
)

type Note struct {
	Order []string `json:"order"`
	Items map[string]Item
}

type Item struct {
	MD    *TextMD `json:"md,omitempty"`
	Shell *Shell  `json:"shell,omitempty"`
}

func emptyNote() Note {
	return Note{Order: make([]string, 0), Items: make(map[string]Item)}
}

// Session holds the state of one browser connection: the open notebook and
// its interpreter. Sessions are independent, so tabs no longer share state.
type Session struct {
	mu     sync.Mutex
	note   Note
	name   string // notebook name, without directory or extension
	interp *fast.Interp
	timer  *time.Timer
}

func newSession(name string, interp *fast.Interp) *Session {
	return &Session{name: name, interp: interp, note: emptyNote()}
}

var notebookNameRe = regexp.MustCompile(`[^a-zA-Z0-9._ -]+`)

// sanitizeNotebookName reduces a user-supplied notebook name to a safe file
// base name. Returns "" when nothing safe remains.
func sanitizeNotebookName(name string) string {
	name = strings.TrimSuffix(filepath.Base(name), ".gonote.json")
	name = notebookNameRe.ReplaceAllString(name, "")
	name = strings.Trim(name, ". ")
	return name
}

func notebookPath(name string) string {
	return filepath.Join("notebooks", name+".gonote.json")
}

func listNotebooks() []string {
	entries, _ := os.ReadDir("notebooks")
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		n := e.Name()
		if strings.HasSuffix(n, ".gonote.json") {
			base := strings.TrimSuffix(n, ".gonote.json")
			if base != "" {
				out = append(out, base)
			}
		}
	}
	sort.Strings(out)
	return out
}

// load switches the session to the given notebook, creating it when missing.
func (s *Session) load(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.name = name
	s.note = emptyNote()
	p := notebookPath(name)
	if !liveview.Exists(p) {
		liveview.StringToFile(p, `{"order":[], "Items":{}}`)
	}
	data, err := os.ReadFile(p)
	if err != nil {
		log.Println("reading notebook:", err)
		return
	}
	if err := json.Unmarshal(data, &s.note); err != nil {
		log.Println("parsing notebook:", err)
	}
}

func (s *Session) notebookName() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.name
}

func (s *Session) snapshotOrder() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, len(s.note.Order))
	copy(out, s.note.Order)
	return out
}

func (s *Session) item(id string) Item {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.note.Items[id]
}

// touch updates (or adds) an item and schedules a save.
func (s *Session) touch(id string, it Item) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !liveview.ContainsString(s.note.Order, id) {
		s.note.Order = append(s.note.Order, id)
	}
	s.note.Items[id] = it
	s.scheduleSaveLocked()
}

func (s *Session) removeItem(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.note.Items, id)
	for i, key := range s.note.Order {
		if key == id {
			s.note.Order = append(s.note.Order[:i], s.note.Order[i+1:]...)
			break
		}
	}
	s.scheduleSaveLocked()
}

// scheduleSaveLocked debounces disk writes: cell edits and reloads produce
// bursts of updates that collapse into a single write.
func (s *Session) scheduleSaveLocked() {
	if s.timer != nil {
		s.timer.Stop()
	}
	s.timer = time.AfterFunc(400*time.Millisecond, s.Flush)
}

// Flush writes the notebook to disk immediately.
func (s *Session) Flush() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.timer != nil {
		s.timer.Stop()
		s.timer = nil
	}
	b, err := json.Marshal(s.note)
	if err != nil {
		log.Println("saving notebook:", err)
		return
	}
	liveview.StringToFile(notebookPath(s.name), string(b))
}
