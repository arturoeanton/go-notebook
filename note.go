package main

import (
	"encoding/json"
	"log"

	"github.com/arturoeanton/go-echo-live-view/liveview"
)

type Note struct {
	Order []string `json:"order"`
	Items map[string]Item
}

type Item struct {
	MD    *TextMD `json:"md,omitempty"`
	Shell *Shell  `json:"shell,omitempty"`
}

var (
	note         Note = Note{Order: make([]string, 0), Items: make(map[string]Item)}
	notebookFile      = "notebooks/default.gonote.json"
)

func saveNote() {
	jsonString, err := json.Marshal(note)
	if err != nil {
		log.Println(err)
		return
	}
	liveview.StringToFile(notebookFile, string(jsonString))
}
