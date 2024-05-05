package web

import "embed"

// DESIGN#2
// In order for this whole thing to compile neatly into one binary with no file dependencies
// The HTML has to get embedded in the binary
// Then they get added to the database
// Can change them in there and see changes instantly
// Except for routes, they need to be reloaded

//go:embed template/*
var starterTemplates embed.FS
