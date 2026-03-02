package parser

import (
	"context"
	"strings"
	"sync"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/golang"
	"github.com/smacker/go-tree-sitter/javascript"
	"github.com/smacker/go-tree-sitter/python"
	tstypescript "github.com/smacker/go-tree-sitter/typescript/typescript"
)

// treeCache stores previously parsed trees for incremental re-parsing.
var (
	treeCacheMu sync.Mutex
	treeCache   = make(map[string]*sitter.Tree) // path → last parsed tree
)

// langGrammar maps our Language type to tree-sitter language grammars.
func langGrammar(lang Language) *sitter.Language {
	switch lang {
	case Python:
		return python.GetLanguage()
	case TypeScript:
		return tstypescript.GetLanguage()
	case JavaScript:
		return javascript.GetLanguage()
	case GoLang:
		return golang.GetLanguage()
	default:
		return nil
	}
}

// parseTSFile parses a source file using tree-sitter and returns a ParsedFile
// with the same interface as the regex parser. Implements incremental re-parsing
// by reusing the previous AST for unchanged subtrees.
func parseTSFile(path string, src []byte, lang Language) (*ParsedFile, error) {
	grammar := langGrammar(lang)
	if grammar == nil {
		return nil, nil
	}

	p := sitter.NewParser()
	p.SetLanguage(grammar)

	// Retrieve old tree for incremental re-parse
	treeCacheMu.Lock()
	oldTree := treeCache[path]
	treeCacheMu.Unlock()

	tree, err := p.ParseCtx(context.Background(), oldTree, src)
	if err != nil {
		return nil, err
	}

	// Cache the new tree
	treeCacheMu.Lock()
	treeCache[path] = tree
	treeCacheMu.Unlock()

	pf := &ParsedFile{
		Path:     path,
		Language: lang,
		Hash:     hashBytes(src),
	}

	switch lang {
	case Python:
		extractPython(pf, tree, src, grammar)
	case TypeScript, JavaScript:
		extractTypeScript(pf, tree, src, grammar)
	case GoLang:
		extractGo(pf, tree, src, grammar)
	}

	return pf, nil
}

// InvalidateTreeCache removes the cached tree for a deleted file.
func InvalidateTreeCache(path string) {
	treeCacheMu.Lock()
	delete(treeCache, path)
	treeCacheMu.Unlock()
}

// ---- Python extraction ----

// pythonSources maps request attribute names to tainted source symbols.
var pythonSources = map[string]bool{
	"args": true, "form": true, "json": true, "data": true, "files": true,
	"cookies": true, "headers": true, "values": true, "query_params": true,
	"path_params": true, "body": true,
}

// pythonSQLSinks maps method names that are SQL sinks.
var pythonSQLSinks = map[string]bool{
	"execute": true, "executemany": true, "executescript": true,
	"scalar": true, "query": true, "exec_driver_sql": true,
}

// pythonRCESinks maps function/method names that are RCE sinks.
var pythonRCESinks = map[string]bool{
	"system": true, "popen": true, "run": true, "call": true,
	"Popen": true, "check_output": true, "check_call": true,
	"eval": true, "exec": true, "compile": true,
}

// pythonDeserSinks for deserialization vulnerabilities.
var pythonDeserSinks = map[string]bool{
	"loads": true, "load": true, "decode": true,
}

// pythonSSRFSinks for SSRF vulnerabilities.
var pythonSSRFSinks = map[string]bool{
	"get": true, "post": true, "put": true, "delete": true,
	"patch": true, "request": true, "head": true, "options": true,
}

// pythonFileSinks for path traversal.
var pythonFileSinks = map[string]bool{
	"open": true, "remove": true, "unlink": true, "rename": true,
	"mkdir": true, "makedirs": true, "listdir": true,
}

// pythonSanitizers maps function names to the vuln class they sanitize.
var pythonSanitizers = map[string]string{
	"escape": "xss", "clean": "xss", "sanitize": "xss",
	"text":          "sqli",
	"filter":        "sqli",
	"filter_by":     "sqli",
	"quote":         "sqli",
	"quote_plus":    "sqli",
	"html_escape":   "xss",
	"bleach":        "xss",
}

// httpReceivers identifies objects that carry HTTP request data.
var httpReceivers = map[string]bool{
	"request": true, "req": true,
}

func extractPython(pf *ParsedFile, tree *sitter.Tree, src []byte, lang *sitter.Language) {
	framework := ""
	handlerFuncs := map[string]bool{} // func names that are HTTP handlers

	// 1. Imports → framework detection
	runQuery(tree, src, lang, pyQImport, func(match *sitter.QueryMatch, query *sitter.Query) {
		for _, cap := range match.Captures {
			capName := query.CaptureNameForId(cap.Index)
			if capName == "pkg" {
				pkg := strings.Trim(cap.Node.Content(src), `"'`)
				pf.Imports = append(pf.Imports, pkg)
				switch {
				case strings.HasPrefix(pkg, "fastapi"):
					framework = "fastapi"
				case strings.HasPrefix(pkg, "django"):
					framework = "django"
				case strings.HasPrefix(pkg, "flask"):
					framework = "flask"
				case strings.HasPrefix(pkg, "starlette") && framework == "":
					framework = "starlette"
				}
			}
		}
	})

	// 2. Decorators → identify handler functions
	runQuery(tree, src, lang, pyQDecorator, func(match *sitter.QueryMatch, query *sitter.Query) {
		var decoObj, decoMethod string
		var decoNode *sitter.Node
		for _, cap := range match.Captures {
			name := query.CaptureNameForId(cap.Index)
			switch name {
			case "deco.obj":
				decoObj = cap.Node.Content(src)
			case "deco.method":
				decoMethod = cap.Node.Content(src)
			case "decorator":
				decoNode = cap.Node
			}
		}
		routeMethods := map[string]bool{
			"get": true, "post": true, "put": true, "patch": true,
			"delete": true, "websocket": true, "route": true,
		}
		if routeMethods[decoMethod] && decoNode != nil {
			// Next sibling should be the function_definition
			next := decoNode.NextNamedSibling()
			if next != nil {
				nameNode := next.ChildByFieldName("name")
				if nameNode != nil {
					handlerFuncs[nameNode.Content(src)] = true
				}
			}
		}
		_ = decoObj
	})

	// 3. Function definitions → FuncDef + handler source params
	runQuery(tree, src, lang, pyQFuncDef, func(match *sitter.QueryMatch, query *sitter.Query) {
		var funcName string
		var paramsNode *sitter.Node
		var funcLine int
		for _, cap := range match.Captures {
			capName := query.CaptureNameForId(cap.Index)
			switch capName {
			case "func.name":
				funcName = cap.Node.Content(src)
				funcLine = int(cap.Node.StartPoint().Row) + 1
			case "func.params":
				paramsNode = cap.Node
			}
		}
		if funcName == "" {
			return
		}
		params := extractPyParams(paramsNode, src)
		isHandler := handlerFuncs[funcName]
		fd := FuncDef{
			Name: funcName, Line: funcLine, Params: params,
			IsHandler: isHandler, Framework: framework,
		}
		pf.Functions = append(pf.Functions, fd)

		// HTTP handler params are sources
		if isHandler {
			for _, p := range params {
				p = strings.TrimSpace(p)
				if p == "" || p == "self" || p == "request" || p == "req" {
					continue
				}
				pf.Nodes = append(pf.Nodes, ParsedNode{
					Kind: KindSource, Symbol: p, Framework: framework,
					Line: funcLine, Text: funcName + "(" + strings.Join(params, ", ") + ")",
					TaintedVars: []string{p},
				})
			}
		}
	})

	// 4. Sources — request attribute access (request.args, request.form, etc.)
	runQuery(tree, src, lang, pyQSource, func(match *sitter.QueryMatch, query *sitter.Query) {
		var recv, attr string
		var line int
		for _, cap := range match.Captures {
			capName := query.CaptureNameForId(cap.Index)
			switch capName {
			case "req":
				recv = cap.Node.Content(src)
				line = int(cap.Node.StartPoint().Row) + 1
			case "attr":
				attr = cap.Node.Content(src)
			}
		}
		if httpReceivers[recv] && pythonSources[attr] {
			sym := recv + "." + attr
			pf.Nodes = append(pf.Nodes, ParsedNode{
				Kind: KindSource, Symbol: sym, Framework: framework,
				Line: line, Text: sym,
				TaintedVars: []string{sym},
			})
		}
	})

	// 5. Sinks — method calls on known objects
	runQuery(tree, src, lang, pyQSink, func(match *sitter.QueryMatch, query *sitter.Query) {
		var obj, method string
		var line int
		var argsText string
		for _, cap := range match.Captures {
			capName := query.CaptureNameForId(cap.Index)
			switch capName {
			case "obj":
				obj = cap.Node.Content(src)
			case "method":
				method = cap.Node.Content(src)
				line = int(cap.Node.StartPoint().Row) + 1
			case "args":
				argsText = cap.Node.Content(src)
			}
		}
		vulnClass := classifyPySink(obj, method)
		if vulnClass != "" {
			pf.Nodes = append(pf.Nodes, ParsedNode{
				Kind: KindSink, Symbol: obj + "." + method,
				VulnClass: vulnClass, Framework: framework,
				Line: line, Text: obj + "." + method + argsText,
			})
		}
		if sanitizerClass, ok := pythonSanitizers[method]; ok {
			pf.Nodes = append(pf.Nodes, ParsedNode{
				Kind: KindSanitizer, Symbol: obj + "." + method,
				VulnClass: sanitizerClass, Framework: framework,
				Line: line, Text: obj + "." + method + argsText,
			})
		}
	})

	// 6. Simple sinks/sanitizers — top-level calls (eval, exec, escape, etc.)
	runQuery(tree, src, lang, pyQSimpleSink, func(match *sitter.QueryMatch, query *sitter.Query) {
		var callee string
		var line int
		for _, cap := range match.Captures {
			capName := query.CaptureNameForId(cap.Index)
			switch capName {
			case "callee":
				callee = cap.Node.Content(src)
				line = int(cap.Node.StartPoint().Row) + 1
			}
		}
		// RCE sinks: top-level eval/exec/compile
		if callee == "eval" || callee == "exec" || callee == "compile" {
			pf.Nodes = append(pf.Nodes, ParsedNode{
				Kind: KindSink, Symbol: callee, VulnClass: "rce",
				Framework: framework, Line: line, Text: callee + "(...)",
			})
		}
		// Sanitizers: top-level escape(), clean(), etc.
		if vc, ok := pythonSanitizers[callee]; ok {
			pf.Nodes = append(pf.Nodes, ParsedNode{
				Kind: KindSanitizer, Symbol: callee, VulnClass: vc,
				Framework: framework, Line: line, Text: callee + "(...)",
			})
		}
	})

	// 7. Call sites → for inter-procedural analysis
	currentFunc := ""
	if len(pf.Functions) > 0 {
		currentFunc = pf.Functions[0].Name
	}
	runQuery(tree, src, lang, pyQCall, func(match *sitter.QueryMatch, query *sitter.Query) {
		var callee string
		var line int
		var argNames []string
		for _, cap := range match.Captures {
			capName := query.CaptureNameForId(cap.Index)
			switch capName {
			case "callee":
				callee = cap.Node.Content(src)
				line = int(cap.Node.StartPoint().Row) + 1
			case "args":
				argNames = extractArgNames(cap.Node, src)
			}
		}
		if callee != "" && !isBuiltin(callee) {
			pf.CallSites = append(pf.CallSites, CallSite{
				CallerFunc: currentFunc,
				Callee:     callee,
				ArgNames:   argNames,
				Line:       line,
			})
		}
	})
}

func classifyPySink(obj, method string) string {
	// SQL objects
	sqlObjs := map[string]bool{
		"cursor": true, "conn": true, "db": true, "session": true,
		"engine": true, "connection": true,
	}
	if sqlObjs[obj] && pythonSQLSinks[method] {
		return "sqli"
	}
	// OS/subprocess objects
	if obj == "os" && (method == "system" || method == "popen") {
		return "rce"
	}
	if obj == "subprocess" && pythonRCESinks[method] {
		return "rce"
	}
	// File operations
	if obj == "os" && pythonFileSinks[method] {
		return "path_traversal"
	}
	if obj == "shutil" {
		return "path_traversal"
	}
	// SSRF
	ssrfObjs := map[string]bool{
		"requests": true, "httpx": true, "aiohttp": true,
		"urllib": true, "session": true,
	}
	if ssrfObjs[obj] && pythonSSRFSinks[method] {
		return "ssrf"
	}
	// Deserialization
	deserObjs := map[string]bool{"pickle": true, "yaml": true, "marshal": true, "jsonpickle": true}
	if deserObjs[obj] && pythonDeserSinks[method] {
		return "deserialization"
	}
	// Unsafe HTML
	unsafeHTML := map[string]bool{"Markup": true, "mark_safe": true, "format_html": true}
	if unsafeHTML[method] {
		return "xss"
	}
	return ""
}

// ---- TypeScript/JavaScript extraction ----

var tsHTTPSources = map[string]bool{
	"body": true, "query": true, "params": true, "headers": true, "cookies": true,
}

var tsRequestReceivers = map[string]bool{
	"req": true, "request": true,
}

// tsDOMSources for DOM-based XSS sources.
var tsDOMSources = map[string]bool{
	"URL": true, "href": true, "hash": true, "search": true, "pathname": true,
}

func extractTypeScript(pf *ParsedFile, tree *sitter.Tree, src []byte, lang *sitter.Language) {
	framework := ""

	// 1. Imports
	runQuery(tree, src, lang, tsQImport, func(match *sitter.QueryMatch, query *sitter.Query) {
		for _, cap := range match.Captures {
			if query.CaptureNameForId(cap.Index) == "pkg" {
				pkg := strings.Trim(cap.Node.Content(src), `"'`)
				pf.Imports = append(pf.Imports, pkg)
				switch {
				case strings.HasPrefix(pkg, "express"):
					framework = "express"
				case strings.HasPrefix(pkg, "next"):
					framework = "nextjs"
				case strings.HasPrefix(pkg, "fastify"):
					framework = "fastify"
				case strings.HasPrefix(pkg, "koa"):
					framework = "koa"
				}
			}
		}
	})

	// 2. Function definitions
	runQuery(tree, src, lang, tsQFuncDef, func(match *sitter.QueryMatch, query *sitter.Query) {
		var funcName string
		var paramsNode *sitter.Node
		var funcLine int
		for _, cap := range match.Captures {
			switch query.CaptureNameForId(cap.Index) {
			case "func.name":
				funcName = cap.Node.Content(src)
				funcLine = int(cap.Node.StartPoint().Row) + 1
			case "func.params":
				paramsNode = cap.Node
			}
		}
		if funcName != "" {
			params := extractTSParams(paramsNode, src)
			pf.Functions = append(pf.Functions, FuncDef{
				Name: funcName, Line: funcLine, Params: params, Framework: framework,
			})
		}
	})

	// 3. Sources — req.body, req.query, etc.
	runQuery(tree, src, lang, tsQSource, func(match *sitter.QueryMatch, query *sitter.Query) {
		var recv, attr string
		var line int
		for _, cap := range match.Captures {
			switch query.CaptureNameForId(cap.Index) {
			case "req":
				recv = cap.Node.Content(src)
				line = int(cap.Node.StartPoint().Row) + 1
			case "attr":
				attr = cap.Node.Content(src)
			}
		}
		if tsRequestReceivers[recv] && tsHTTPSources[attr] {
			sym := recv + "." + attr
			pf.Nodes = append(pf.Nodes, ParsedNode{
				Kind: KindSource, Symbol: sym, Framework: framework,
				Line: line, Text: sym,
				TaintedVars: []string{sym},
			})
		}
		// DOM sources
		if (recv == "location" || recv == "document") && tsDOMSources[attr] {
			sym := recv + "." + attr
			pf.Nodes = append(pf.Nodes, ParsedNode{
				Kind: KindSource, Symbol: sym, Framework: framework,
				Line: line, Text: sym,
				TaintedVars: []string{sym},
			})
		}
	})

	// 4. Sinks & sanitizers — method calls
	runQuery(tree, src, lang, tsQCall, func(match *sitter.QueryMatch, query *sitter.Query) {
		var callee string
		var line int
		var argNames []string
		for _, cap := range match.Captures {
			switch query.CaptureNameForId(cap.Index) {
			case "callee":
				callee = cap.Node.Content(src)
				line = int(cap.Node.StartPoint().Row) + 1
			case "args":
				argNames = extractArgNames(cap.Node, src)
			}
		}
		// Classify sinks
		if vulnClass := classifyTSSink(callee); vulnClass != "" {
			pf.Nodes = append(pf.Nodes, ParsedNode{
				Kind: KindSink, Symbol: callee, VulnClass: vulnClass,
				Framework: framework, Line: line,
			})
		}
		// Sanitizers
		if vc := classifyTSSanitizer(callee); vc != "" {
			pf.Nodes = append(pf.Nodes, ParsedNode{
				Kind: KindSanitizer, Symbol: callee, VulnClass: vc,
				Framework: framework, Line: line,
			})
		}
		// Call sites
		if callee != "" && !isBuiltin(callee) {
			pf.CallSites = append(pf.CallSites, CallSite{
				Callee: callee, ArgNames: argNames, Line: line,
			})
		}
	})
}

func classifyTSSink(callee string) string {
	switch callee {
	case "innerHTML", "outerHTML", "write", "dangerouslySetInnerHTML":
		return "xss"
	case "eval":
		return "rce"
	case "exec", "execSync", "spawn", "spawnSync", "execFile":
		return "rce"
	case "readFile", "readFileSync", "writeFile", "writeFileSync", "appendFile", "unlink", "rm":
		return "path_traversal"
	case "fetch":
		return "ssrf"
	}
	return ""
}

func classifyTSSanitizer(callee string) string {
	switch callee {
	case "sanitize": // DOMPurify.sanitize
		return "xss"
	case "encodeURIComponent", "encodeURI":
		return "xss"
	case "escape":
		return "xss"
	case "prepare":
		return "sqli"
	}
	return ""
}

// ---- Go extraction ----

var goHTTPSources = map[string]bool{
	"FormValue": true, "PostFormValue": true, "Header": true,
	"Cookie": true, "URLParam": true, "PathValue": true,
	"Param": true, "Query": true, "PostForm": true,
	"GetRawData": true, "ShouldBind": true, "Bind": true,
}

var goSQLSinkMethods = map[string]bool{
	"Query": true, "QueryRow": true, "Exec": true,
	"QueryContext": true, "ExecContext": true, "QueryRowContext": true,
}

var goFileSinkMethods = map[string]bool{
	"Open": true, "OpenFile": true, "Create": true,
	"ReadFile": true, "WriteFile": true, "Remove": true, "Rename": true,
}

var goRCESinkMethods = map[string]bool{
	"Command": true, "StartProcess": true,
}

var goSSRFSinkMethods = map[string]bool{
	"Get": true, "Post": true, "Head": true, "Do": true,
}

// goSSRFReceivers are package-level identifiers that make HTTP calls.
var goSSRFReceivers = map[string]bool{
	"http": true, "client": true,
}

func extractGo(pf *ParsedFile, tree *sitter.Tree, src []byte, lang *sitter.Language) {
	framework := ""

	// 1. Imports
	runQuery(tree, src, lang, goQImport, func(match *sitter.QueryMatch, query *sitter.Query) {
		for _, cap := range match.Captures {
			if query.CaptureNameForId(cap.Index) == "pkg" {
				pkg := strings.Trim(cap.Node.Content(src), `"`)
				pf.Imports = append(pf.Imports, pkg)
				switch {
				case strings.Contains(pkg, "go-chi/chi"):
					framework = "chi"
				case strings.Contains(pkg, "gin-gonic/gin"):
					framework = "gin"
				}
			}
		}
	})

	// 2. Function definitions
	runQuery(tree, src, lang, goQFuncDef, func(match *sitter.QueryMatch, query *sitter.Query) {
		var funcName string
		var paramsNode *sitter.Node
		var funcLine int
		for _, cap := range match.Captures {
			switch query.CaptureNameForId(cap.Index) {
			case "func.name":
				funcName = cap.Node.Content(src)
				funcLine = int(cap.Node.StartPoint().Row) + 1
			case "func.params":
				paramsNode = cap.Node
			}
		}
		if funcName != "" {
			params := extractGoParamsFromNode(paramsNode, src)
			isHandler := isGoHTTPHandler(paramsNode, src)
			pf.Functions = append(pf.Functions, FuncDef{
				Name: funcName, Line: funcLine, Params: params,
				IsHandler: isHandler, Framework: framework,
			})
		}
	})

	// 3. Sources and sinks — method calls (covers both direct and chained: r.URL.Query().Get)
	runQuery(tree, src, lang, goQSource, func(match *sitter.QueryMatch, query *sitter.Query) {
		var recv, method string
		var line int
		for _, cap := range match.Captures {
			switch query.CaptureNameForId(cap.Index) {
			case "recv":
				recv = cap.Node.Content(src)
			case "method":
				method = cap.Node.Content(src)
				line = int(cap.Node.StartPoint().Row) + 1
			}
		}

		sym := recv + "." + method

		// HTTP sources: direct (r.FormValue) or chained (r.URL.Query().Get)
		if goHTTPSources[method] {
			pf.Nodes = append(pf.Nodes, ParsedNode{
				Kind: KindSource, Symbol: sym, Framework: framework,
				Line: line, Text: sym, TaintedVars: []string{recv},
			})
			return
		}
		// Chained HTTP source: anything.Get(...) where chain contains URL.Query
		if method == "Get" && strings.Contains(recv, "URL.Query") {
			pf.Nodes = append(pf.Nodes, ParsedNode{
				Kind: KindSource, Symbol: sym, Framework: framework,
				Line: line, Text: sym, TaintedVars: []string{recv},
			})
			return
		}

		// SQL sinks
		if goSQLSinkMethods[method] {
			pf.Nodes = append(pf.Nodes, ParsedNode{
				Kind: KindSink, Symbol: sym, VulnClass: "sqli",
				Framework: framework, Line: line,
			})
		}
		// File / path traversal sinks
		if goFileSinkMethods[method] {
			pf.Nodes = append(pf.Nodes, ParsedNode{
				Kind: KindSink, Symbol: sym, VulnClass: "path_traversal",
				Framework: framework, Line: line,
			})
		}
		// RCE sinks: exec.Command, os.StartProcess
		if goRCESinkMethods[method] {
			pf.Nodes = append(pf.Nodes, ParsedNode{
				Kind: KindSink, Symbol: sym, VulnClass: "rce",
				Framework: framework, Line: line,
			})
		}
		// SSRF sinks: http.Get, http.Post, client.Do
		if goSSRFSinkMethods[method] && goSSRFReceivers[recv] {
			pf.Nodes = append(pf.Nodes, ParsedNode{
				Kind: KindSink, Symbol: sym, VulnClass: "ssrf",
				Framework: framework, Line: line,
			})
		}
		// XSS: template.HTML unsafe cast
		if method == "HTML" && recv == "template" {
			pf.Nodes = append(pf.Nodes, ParsedNode{
				Kind: KindSink, Symbol: sym, VulnClass: "xss",
				Framework: framework, Line: line,
			})
		}
		// Sanitizers — HTML escaping
		if method == "EscapeString" || method == "HTMLEscapeString" {
			pf.Nodes = append(pf.Nodes, ParsedNode{
				Kind: KindSanitizer, Symbol: sym, VulnClass: "xss",
				Framework: framework, Line: line,
			})
		}
	})

	// 4. Call sites
	runQuery(tree, src, lang, goQCall, func(match *sitter.QueryMatch, query *sitter.Query) {
		var callee string
		var line int
		var argNames []string
		for _, cap := range match.Captures {
			switch query.CaptureNameForId(cap.Index) {
			case "callee":
				callee = cap.Node.Content(src)
				line = int(cap.Node.StartPoint().Row) + 1
			case "args":
				argNames = extractArgNames(cap.Node, src)
			}
		}
		if callee != "" && !isBuiltin(callee) {
			pf.CallSites = append(pf.CallSites, CallSite{
				Callee: callee, ArgNames: argNames, Line: line,
			})
		}
	})
}

// ---- Query execution helper ----

// runQuery executes a tree-sitter query and calls fn for each match.
func runQuery(tree *sitter.Tree, src []byte, lang *sitter.Language, queryStr string, fn func(*sitter.QueryMatch, *sitter.Query)) {
	q, err := sitter.NewQuery([]byte(queryStr), lang)
	if err != nil {
		return // malformed query — skip
	}
	defer q.Close()

	c := sitter.NewQueryCursor()
	defer c.Close()
	c.Exec(q, tree.RootNode())

	for {
		m, ok := c.NextMatch()
		if !ok {
			break
		}
		fn(m, q)
	}
}

// ---- AST helpers ----

// extractPyParams extracts parameter names from a Python parameters node.
func extractPyParams(node *sitter.Node, src []byte) []string {
	if node == nil {
		return nil
	}
	var out []string
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		t := child.Type()
		if t == "identifier" {
			name := child.Content(src)
			if name != "self" && name != "cls" {
				out = append(out, name)
			}
		} else if t == "typed_parameter" || t == "default_parameter" || t == "typed_default_parameter" {
			nameNode := child.ChildByFieldName("name")
			if nameNode != nil {
				out = append(out, nameNode.Content(src))
			}
		}
	}
	return out
}

// extractTSParams extracts parameter names from a TypeScript formal_parameters node.
func extractTSParams(node *sitter.Node, src []byte) []string {
	if node == nil {
		return nil
	}
	var out []string
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		switch child.Type() {
		case "identifier":
			out = append(out, child.Content(src))
		case "required_parameter", "optional_parameter":
			nameNode := child.ChildByFieldName("pattern")
			if nameNode != nil {
				out = append(out, nameNode.Content(src))
			}
		}
	}
	return out
}

// extractGoParamsFromNode extracts parameter names from a Go parameter_list node.
func extractGoParamsFromNode(node *sitter.Node, src []byte) []string {
	if node == nil {
		return nil
	}
	var out []string
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "parameter_declaration" {
			nameNode := child.ChildByFieldName("name")
			if nameNode != nil {
				out = append(out, nameNode.Content(src))
			}
		}
	}
	return out
}

// isGoHTTPHandler returns true if the parameter list matches (w http.ResponseWriter, r *http.Request).
func isGoHTTPHandler(params *sitter.Node, src []byte) bool {
	if params == nil {
		return false
	}
	content := params.Content(src)
	return strings.Contains(content, "http.ResponseWriter") && strings.Contains(content, "http.Request")
}

// extractArgNames extracts variable names used as arguments in a call.
func extractArgNames(argsNode *sitter.Node, src []byte) []string {
	if argsNode == nil {
		return nil
	}
	var out []string
	for i := 0; i < int(argsNode.ChildCount()); i++ {
		child := argsNode.Child(i)
		if child.Type() == "identifier" {
			out = append(out, child.Content(src))
		}
	}
	return out
}

// isBuiltin returns true for built-in functions we don't want in the call graph.
func isBuiltin(name string) bool {
	builtins := map[string]bool{
		// Python + Go overlap (deduplicated)
		"print": true, "len": true, "range": true, "str": true, "int": true,
		"float": true, "list": true, "dict": true, "set": true, "tuple": true,
		"type": true, "isinstance": true, "hasattr": true, "getattr": true,
		"setattr": true, "append": true, "extend": true, "format": true,
		"super": true, "property": true, "staticmethod": true, "classmethod": true,
		// TypeScript/JS
		"console": true, "setTimeout": true, "setInterval": true, "clearTimeout": true,
		"parseInt": true, "parseFloat": true, "JSON": true, "Object": true,
		"Array": true, "Promise": true, "Math": true, "Date": true,
		// Go-only
		"make": true, "new": true, "copy": true, "delete": true,
		"close": true, "panic": true, "recover": true, "cap": true,
	}
	return builtins[name]
}
