package template

import "embed"

//go:embed conf/* block/*
var DistFS embed.FS
