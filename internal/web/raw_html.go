package web

import "embed"

// DESIGN#2
// In order for this whole thing to compile neatly into one binary with no file dependencies
// The templates, json config, etc. are all here in string literals
// Then they get added to the database
// Can change them in there and see changes instantly
// Except for routes, they need to be reloaded

// HTML templates
// These are inserted into the database in table called 'templates'
// They're all in their own files now, much nicer right?

//go:embed template/*
var starterTemplates embed.FS
