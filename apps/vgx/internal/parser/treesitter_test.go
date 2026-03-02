package parser

import (
	"testing"
)

func TestTSParsePythonSources(t *testing.T) {
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
		t.Fatal("expected at least one SourceNode from FastAPI handler param")
	}
	if sources[0].Framework != "fastapi" {
		t.Errorf("expected framework fastapi, got %q", sources[0].Framework)
	}
}

func TestTSParsePythonSinks(t *testing.T) {
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
	found := false
	for _, s := range sinks {
		if s.VulnClass == "rce" {
			found = true
		}
	}
	if !found {
		t.Error("expected at least one rce SinkNode")
	}
}

func TestTSParsePythonSanitizer(t *testing.T) {
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

func TestTSParsePythonCallSites(t *testing.T) {
	code := `def process(data):
    result = validate(data)
    return result

def validate(x):
    return x.strip()
`
	pf := parseString(t, code, ".py")
	if pf == nil {
		t.Fatal("ParseFile returned nil")
	}

	if len(pf.CallSites) == 0 {
		t.Fatal("expected at least one CallSite from validate(data)")
	}
	found := false
	for _, cs := range pf.CallSites {
		if cs.Callee == "validate" {
			found = true
		}
	}
	if !found {
		t.Error("expected CallSite with Callee=validate")
	}
}

func TestTSParsePythonFunctions(t *testing.T) {
	code := `def handle_request(user_id, name):
    pass

async def fetch_data(url):
    pass
`
	pf := parseString(t, code, ".py")
	if pf == nil {
		t.Fatal("ParseFile returned nil")
	}

	if len(pf.Functions) < 2 {
		t.Fatalf("expected at least 2 FuncDefs, got %d", len(pf.Functions))
	}
	names := map[string]bool{}
	for _, f := range pf.Functions {
		names[f.Name] = true
	}
	if !names["handle_request"] {
		t.Error("expected FuncDef for handle_request")
	}
	if !names["fetch_data"] {
		t.Error("expected FuncDef for fetch_data")
	}
}

func TestTSParseGoSourceChained(t *testing.T) {
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
		t.Fatal("expected at least one SourceNode from r.URL.Query().Get(...)")
	}
}

func TestTSParseGoRCESink(t *testing.T) {
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

func TestTSParseGoFunctions(t *testing.T) {
	code := `package main

func processInput(userID string, data []byte) error {
	return nil
}

func (s *Server) handleRequest(w http.ResponseWriter, r *http.Request) {
}
`
	pf := parseString(t, code, ".go")
	if pf == nil {
		t.Fatal("ParseFile returned nil")
	}

	if len(pf.Functions) == 0 {
		t.Fatal("expected at least one FuncDef")
	}
	found := false
	for _, f := range pf.Functions {
		if f.Name == "processInput" {
			found = true
			if len(f.Params) == 0 {
				t.Error("expected params for processInput")
			}
		}
	}
	if !found {
		t.Error("expected FuncDef for processInput")
	}
}

func TestTSParseTypeScriptSources(t *testing.T) {
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

func TestTSParseTypeScriptSinks(t *testing.T) {
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
	found := false
	for _, s := range sinks {
		if s.VulnClass == "rce" {
			found = true
		}
	}
	if !found {
		t.Error("expected rce SinkNode")
	}
}
