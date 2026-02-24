package parser

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectLanguage(t *testing.T) {
	tests := []struct {
		path string
		want Language
	}{
		{"app.py", Python},
		{"main.go", GoLang},
		{"server.ts", TypeScript},
		{"index.js", JavaScript},
		{"readme.md", Unknown},
		{"image.png", Unknown},
	}
	for _, tt := range tests {
		got := DetectLanguage(tt.path)
		if got != tt.want {
			t.Errorf("DetectLanguage(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestParsePythonSources(t *testing.T) {
	code := `from fastapi import Query

@app.get("/users")
def get_users(name: str = Query(...)):
    return db.find(name)
`
	pf := parseString(t, code, ".py")
	if pf == nil {
		t.Fatal("ParseFile returned nil")
	}

	var sources []ParsedNode
	for _, n := range pf.Nodes {
		if n.Kind == KindSource {
			sources = append(sources, n)
		}
	}
	if len(sources) == 0 {
		t.Fatal("expected at least one SourceNode from Query(...)")
	}
	if sources[0].Framework != "fastapi" {
		t.Errorf("expected framework fastapi, got %q", sources[0].Framework)
	}
}

func TestParsePythonSinks(t *testing.T) {
	code := `import subprocess

def run_cmd(cmd):
    subprocess.run(cmd, shell=True)
`
	pf := parseString(t, code, ".py")
	if pf == nil {
		t.Fatal("ParseFile returned nil")
	}

	var sinks []ParsedNode
	for _, n := range pf.Nodes {
		if n.Kind == KindSink {
			sinks = append(sinks, n)
		}
	}
	if len(sinks) == 0 {
		t.Fatal("expected at least one SinkNode from subprocess.run")
	}
	if sinks[0].VulnClass != "rce" {
		t.Errorf("expected vuln_class rce, got %q", sinks[0].VulnClass)
	}
}

func TestParsePythonSanitizers(t *testing.T) {
	code := `from markupsafe import escape

def safe(val):
    return escape(val)
`
	pf := parseString(t, code, ".py")
	if pf == nil {
		t.Fatal("ParseFile returned nil")
	}

	var sanitizers []ParsedNode
	for _, n := range pf.Nodes {
		if n.Kind == KindSanitizer {
			sanitizers = append(sanitizers, n)
		}
	}
	if len(sanitizers) == 0 {
		t.Fatal("expected at least one SanitizerNode from escape()")
	}
}

func TestParseGoSources(t *testing.T) {
	code := `package main

import "net/http"

func handler(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	_ = name
}
`
	pf := parseString(t, code, ".go")
	if pf == nil {
		t.Fatal("ParseFile returned nil")
	}

	var sources []ParsedNode
	for _, n := range pf.Nodes {
		if n.Kind == KindSource {
			sources = append(sources, n)
		}
	}
	if len(sources) == 0 {
		t.Fatal("expected at least one SourceNode from r.URL.Query()")
	}
}

func TestParseGoSinks(t *testing.T) {
	code := `package main

import "os/exec"

func run(cmd string) {
	exec.Command(cmd)
}
`
	pf := parseString(t, code, ".go")
	if pf == nil {
		t.Fatal("ParseFile returned nil")
	}

	var sinks []ParsedNode
	for _, n := range pf.Nodes {
		if n.Kind == KindSink {
			sinks = append(sinks, n)
		}
	}
	if len(sinks) == 0 {
		t.Fatal("expected at least one SinkNode from exec.Command")
	}
	if sinks[0].VulnClass != "rce" {
		t.Errorf("expected vuln_class rce, got %q", sinks[0].VulnClass)
	}
}

func TestParseTypeScriptSources(t *testing.T) {
	code := `import express from 'express';

app.get('/users', (req, res) => {
    const name = req.query.name;
    res.send(name);
});
`
	pf := parseString(t, code, ".ts")
	if pf == nil {
		t.Fatal("ParseFile returned nil")
	}

	var sources []ParsedNode
	for _, n := range pf.Nodes {
		if n.Kind == KindSource {
			sources = append(sources, n)
		}
	}
	if len(sources) == 0 {
		t.Fatal("expected at least one SourceNode from req.query")
	}
}

func TestParseTypeScriptSinks(t *testing.T) {
	code := `const { exec } = require('child_process');

function run(input) {
    child_process.exec(input);
}
`
	pf := parseString(t, code, ".ts")
	if pf == nil {
		t.Fatal("ParseFile returned nil")
	}

	var sinks []ParsedNode
	for _, n := range pf.Nodes {
		if n.Kind == KindSink {
			sinks = append(sinks, n)
		}
	}
	if len(sinks) == 0 {
		t.Fatal("expected at least one SinkNode from child_process.exec")
	}
	if sinks[0].VulnClass != "rce" {
		t.Errorf("expected vuln_class rce, got %q", sinks[0].VulnClass)
	}
}

func TestParseFileHash(t *testing.T) {
	code := `x = 1`
	pf := parseString(t, code, ".py")
	if pf == nil {
		t.Fatal("ParseFile returned nil")
	}
	if pf.Hash == "" {
		t.Error("expected non-empty file hash")
	}
}

func TestParseUnsupportedFile(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "readme.md")
	os.WriteFile(p, []byte("# Hello"), 0o644)

	pf, err := ParseFile(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pf != nil {
		t.Error("expected nil for unsupported file")
	}
}

// parseString writes code to a temp file and parses it.
func parseString(t *testing.T, code, ext string) *ParsedFile {
	t.Helper()
	tmp := t.TempDir()
	p := filepath.Join(tmp, "test"+ext)
	if err := os.WriteFile(p, []byte(code), 0o644); err != nil {
		t.Fatal(err)
	}
	pf, err := ParseFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return pf
}
