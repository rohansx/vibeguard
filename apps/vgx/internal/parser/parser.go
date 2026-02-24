// Package parser extracts security-relevant constructs from source files.
// Phase 1 uses pattern-based analysis; tree-sitter incremental parsing is
// a Phase 1.5 upgrade (identical interface, swappable implementation).
package parser

import (
	"bufio"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Language of a source file.
type Language string

const (
	Python     Language = "python"
	TypeScript Language = "typescript"
	JavaScript Language = "javascript"
	GoLang     Language = "go"
	Unknown    Language = "unknown"
)

// NodeKind is the security-semantic role of a parsed construct.
type NodeKind string

const (
	KindSource    NodeKind = "SourceNode"
	KindSink      NodeKind = "SinkNode"
	KindSanitizer NodeKind = "SanitizerNode"
	KindCallSite  NodeKind = "CallSite" // unclassified call — may be source/sink after classification
	KindFuncDef   NodeKind = "FunctionDef"
	KindImport    NodeKind = "Import"
)

// ParsedNode is a security-relevant construct extracted from source code.
type ParsedNode struct {
	Kind        NodeKind
	Symbol      string // function/method/variable name
	VulnClass   string // sqli | xss | rce | path_traversal | ssrf | deserialization
	Framework   string // fastapi | django | express | gin | nextjs | chi
	Line        int
	Text        string   // raw matched line
	TaintedVars []string // variable names that are tainted at this node
}

// ParsedFile is the full extraction result for one source file.
type ParsedFile struct {
	Path      string
	Language  Language
	Hash      string // SHA-256 of file content
	Nodes     []ParsedNode
	Imports   []string // detected framework/library imports
	Functions []FuncDef
}

// FuncDef tracks a function definition for inter-procedural analysis.
type FuncDef struct {
	Name       string
	Line       int
	Params     []string // parameter names
	IsHandler  bool     // is this an HTTP route handler?
	Framework  string
}

// ParseFile parses a source file and extracts security-relevant nodes.
func ParseFile(path string) (*ParsedFile, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	lang := DetectLanguage(path)
	if lang == Unknown {
		return nil, nil // silently skip unsupported files
	}

	hash := fmt.Sprintf("%x", sha256.Sum256(content))
	pf := &ParsedFile{
		Path:     path,
		Language: lang,
		Hash:     hash,
	}

	lines := splitLines(string(content))

	switch lang {
	case Python:
		parsePython(pf, lines)
	case TypeScript, JavaScript:
		parseTypeScript(pf, lines)
	case GoLang:
		parseGo(pf, lines)
	}

	return pf, nil
}

// DetectLanguage infers the programming language from the file extension.
func DetectLanguage(path string) Language {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".py":
		return Python
	case ".ts", ".tsx":
		return TypeScript
	case ".js", ".jsx", ".mjs", ".cjs":
		return JavaScript
	case ".go":
		return GoLang
	default:
		return Unknown
	}
}

// SupportedExtensions returns the file extensions this parser handles.
func SupportedExtensions() []string {
	return []string{".py", ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs", ".go"}
}

// ---- Python patterns ----

var (
	pyImportRe       = regexp.MustCompile(`(?:^|\s)(?:import|from)\s+([\w.]+)`)
	pyFastAPIRoute   = regexp.MustCompile(`@(?:app|router)\.(get|post|put|patch|delete|websocket)\(`)
	pyDjangoView     = regexp.MustCompile(`def\s+\w+\s*\(\s*(?:self,\s*)?request(?:\s*:\s*\w+)?`)
	pyFuncDef        = regexp.MustCompile(`^(?:async\s+)?def\s+(\w+)\s*\(([^)]*)\)`)
	pySQLSink        = regexp.MustCompile(`(?:cursor|conn|db|session|engine)\.(?:execute|executemany|executescript|scalar|query|exec_driver_sql)\s*\(`)
	pyRCESink        = regexp.MustCompile(`(?:os\.system|os\.popen|subprocess\.(?:run|call|Popen|check_output|check_call)|eval|exec|compile)\s*\(`)
	pyPathSink       = regexp.MustCompile(`(?:open|os\.(?:remove|unlink|rename|mkdir|makedirs|listdir)|shutil\.\w+|pathlib\.Path)\s*\(`)
	pyDeserSink      = regexp.MustCompile(`(?:pickle\.loads|pickle\.load|yaml\.load|marshal\.loads|jsonpickle\.decode)\s*\(`)
	pySSRFSink       = regexp.MustCompile(`(?:requests\.|httpx\.|urllib\.request\.|aiohttp\.ClientSession\(\)\.)(?:get|post|put|delete|patch|request|head|options)\s*\(`)
	pyHTMLSink       = regexp.MustCompile(`(?:Markup|mark_safe|format_html|jinja2\.Markup)\s*\(`)
	pySQLSanitizer   = regexp.MustCompile(`(?:text\(\)|bindparams|sqlalchemy\.text|\.filter\(|\.filter_by\(|paramstyle)`)
	pyHTMLSanitizer  = regexp.MustCompile(`(?:html\.escape|bleach\.clean|markupsafe\.escape|escape\()`)
	pyRequestParam   = regexp.MustCompile(`request\.(body|json|data|form|args|query_params|path_params|headers|cookies|files)`)
	pyFastAPIParam   = regexp.MustCompile(`(?:Query|Path|Body|Header|Cookie|Form|File)\(`)
	pyFlaskParam     = regexp.MustCompile(`request\.(args|form|json|data|files|cookies|headers|values)`)
)

func parsePython(pf *ParsedFile, lines []string) {
	framework := ""
	var handlerLines []int

	for i, line := range lines {
		lineno := i + 1
		trimmed := strings.TrimSpace(line)

		// Detect imports / framework
		if m := pyImportRe.FindStringSubmatch(trimmed); m != nil {
			pkg := m[1]
			pf.Imports = append(pf.Imports, pkg)
			switch {
			case strings.HasPrefix(pkg, "fastapi"):
				framework = "fastapi"
			case strings.HasPrefix(pkg, "django"):
				framework = "django"
			case strings.HasPrefix(pkg, "flask"):
				framework = "flask"
			case strings.HasPrefix(pkg, "starlette"):
				if framework == "" {
					framework = "starlette"
				}
			}
			pf.Nodes = append(pf.Nodes, ParsedNode{
				Kind: KindImport, Symbol: pkg, Line: lineno, Text: trimmed,
			})
		}

		// Route decorators — mark next function as handler
		if pyFastAPIRoute.MatchString(trimmed) || strings.Contains(trimmed, "@app.route") || strings.Contains(trimmed, "@blueprint.route") {
			handlerLines = append(handlerLines, lineno)
		}

		// Function definitions
		if m := pyFuncDef.FindStringSubmatch(trimmed); m != nil {
			fname := m[1]
			params := extractPythonParams(m[2])
			isHandler := len(handlerLines) > 0 && handlerLines[len(handlerLines)-1] == lineno-1

			fd := FuncDef{Name: fname, Line: lineno, Params: params, IsHandler: isHandler, Framework: framework}
			pf.Functions = append(pf.Functions, fd)

			if isHandler {
				// Annotate HTTP source params
				for _, p := range params {
					pname := strings.TrimSpace(strings.Split(p, ":")[0])
					if pname == "self" || pname == "request" || pname == "req" || pname == "" {
						continue
					}
					pf.Nodes = append(pf.Nodes, ParsedNode{
						Kind:        KindSource,
						Symbol:      pname,
						Framework:   framework,
						Line:        lineno,
						Text:        trimmed,
						TaintedVars: []string{pname},
					})
				}
			}
		}

		// Django/Flask HTTP source parameters
		if pyDjangoView.MatchString(trimmed) {
			pf.Nodes = append(pf.Nodes, ParsedNode{
				Kind: KindSource, Symbol: "request", Framework: "django",
				Line: lineno, Text: trimmed, TaintedVars: []string{"request"},
			})
		}
		if pyRequestParam.MatchString(trimmed) {
			m := pyRequestParam.FindStringSubmatch(trimmed)
			pf.Nodes = append(pf.Nodes, ParsedNode{
				Kind: KindSource, Symbol: "request." + m[1], Framework: framework,
				Line: lineno, Text: trimmed,
				TaintedVars: []string{extractAssignTarget(trimmed)},
			})
		}
		if pyFlaskParam.MatchString(trimmed) {
			m := pyFlaskParam.FindStringSubmatch(trimmed)
			pf.Nodes = append(pf.Nodes, ParsedNode{
				Kind: KindSource, Symbol: "request." + m[1], Framework: "flask",
				Line: lineno, Text: trimmed,
				TaintedVars: []string{extractAssignTarget(trimmed)},
			})
		}

		// SQL sinks
		if pySQLSink.MatchString(trimmed) {
			pf.Nodes = append(pf.Nodes, ParsedNode{
				Kind: KindSink, Symbol: extractCallName(trimmed, pySQLSink),
				VulnClass: "sqli", Framework: framework,
				Line: lineno, Text: trimmed,
			})
		}
		// RCE sinks
		if pyRCESink.MatchString(trimmed) {
			pf.Nodes = append(pf.Nodes, ParsedNode{
				Kind: KindSink, Symbol: extractCallName(trimmed, pyRCESink),
				VulnClass: "rce", Framework: framework,
				Line: lineno, Text: trimmed,
			})
		}
		// Path traversal sinks
		if pyPathSink.MatchString(trimmed) {
			pf.Nodes = append(pf.Nodes, ParsedNode{
				Kind: KindSink, Symbol: extractCallName(trimmed, pyPathSink),
				VulnClass: "path_traversal", Framework: framework,
				Line: lineno, Text: trimmed,
			})
		}
		// Deserialization sinks
		if pyDeserSink.MatchString(trimmed) {
			pf.Nodes = append(pf.Nodes, ParsedNode{
				Kind: KindSink, Symbol: extractCallName(trimmed, pyDeserSink),
				VulnClass: "deserialization", Framework: framework,
				Line: lineno, Text: trimmed,
			})
		}
		// SSRF sinks
		if pySSRFSink.MatchString(trimmed) {
			pf.Nodes = append(pf.Nodes, ParsedNode{
				Kind: KindSink, Symbol: extractCallName(trimmed, pySSRFSink),
				VulnClass: "ssrf", Framework: framework,
				Line: lineno, Text: trimmed,
			})
		}
		// Unsafe HTML sinks
		if pyHTMLSink.MatchString(trimmed) {
			pf.Nodes = append(pf.Nodes, ParsedNode{
				Kind: KindSink, Symbol: extractCallName(trimmed, pyHTMLSink),
				VulnClass: "xss", Framework: framework,
				Line: lineno, Text: trimmed,
			})
		}
		// Sanitizers
		if pySQLSanitizer.MatchString(trimmed) {
			pf.Nodes = append(pf.Nodes, ParsedNode{
				Kind: KindSanitizer, Symbol: extractCallName(trimmed, pySQLSanitizer),
				VulnClass: "sqli", Framework: framework,
				Line: lineno, Text: trimmed,
			})
		}
		if pyHTMLSanitizer.MatchString(trimmed) {
			pf.Nodes = append(pf.Nodes, ParsedNode{
				Kind: KindSanitizer, Symbol: extractCallName(trimmed, pyHTMLSanitizer),
				VulnClass: "xss", Framework: framework,
				Line: lineno, Text: trimmed,
			})
		}
	}
}

// ---- TypeScript / JavaScript patterns ----

var (
	tsImportRe      = regexp.MustCompile(`(?:import|require)\s*(?:\{[^}]*\}|[\w*]+|\(['"])\s*(?:from\s*)?['"]([^'"]+)['"]`)
	tsExpressRoute  = regexp.MustCompile(`(?:app|router)\.(get|post|put|patch|delete|all|use)\s*\(`)
	tsNextHandler   = regexp.MustCompile(`export\s+(?:async\s+)?(?:default\s+)?function\s+(?:GET|POST|PUT|DELETE|PATCH|handler)\s*\(`)
	tsReqSource     = regexp.MustCompile(`req\.(body|query|params|headers|cookies)`)
	tsNextSource    = regexp.MustCompile(`(?:request\.json\(\)|params\.|searchParams\.|request\.headers)`)
	tsFetchSrc      = regexp.MustCompile(`(?:document\.URL|location\.(?:href|hash|search|pathname)|URLSearchParams)`)
	tsInnerHTMLSink = regexp.MustCompile(`\.innerHTML\s*=|\.outerHTML\s*=|document\.write\s*\(`)
	tsDangSink      = regexp.MustCompile(`dangerouslySetInnerHTML`)
	tsEvalSink      = regexp.MustCompile(`(?:^|[^a-zA-Z.])eval\s*\(`)
	tsDocWriteSink  = regexp.MustCompile(`document\.write\s*\(`)
	tsNodeExecSink  = regexp.MustCompile(`child_process\.(?:exec|execSync|spawn|spawnSync|execFile)\s*\(`)
	tsFSSink        = regexp.MustCompile(`fs\.(?:readFile|readFileSync|writeFile|writeFileSync|appendFile|unlink|rm)\s*\(`)
	tsFetchSink     = regexp.MustCompile(`(?:^|[^a-zA-Z])fetch\s*\(|axios\.(?:get|post|put|delete|request)\s*\(`)
	tsDOMPurify     = regexp.MustCompile(`DOMPurify\.sanitize\s*\(`)
	tsEncodeURI     = regexp.MustCompile(`encodeURIComponent\s*\(|encodeURI\s*\(`)
	tsPreparedStmt  = regexp.MustCompile(`\.prepare\s*\(|parameterized|bindParam|\\$\d+`)
)

func parseTypeScript(pf *ParsedFile, lines []string) {
	framework := ""

	for i, line := range lines {
		lineno := i + 1
		trimmed := strings.TrimSpace(line)

		// Detect imports
		if m := tsImportRe.FindStringSubmatch(trimmed); m != nil {
			pkg := m[1]
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
			pf.Nodes = append(pf.Nodes, ParsedNode{
				Kind: KindImport, Symbol: pkg, Line: lineno, Text: trimmed,
			})
		}

		// HTTP sources — req.body / req.query / req.params
		if tsReqSource.MatchString(trimmed) {
			m := tsReqSource.FindStringSubmatch(trimmed)
			pf.Nodes = append(pf.Nodes, ParsedNode{
				Kind: KindSource, Symbol: "req." + m[1], Framework: framework,
				Line: lineno, Text: trimmed,
				TaintedVars: []string{extractAssignTarget(trimmed), "req." + m[1]},
			})
		}
		// Next.js sources
		if tsNextSource.MatchString(trimmed) {
			pf.Nodes = append(pf.Nodes, ParsedNode{
				Kind: KindSource, Symbol: "request", Framework: "nextjs",
				Line: lineno, Text: trimmed,
				TaintedVars: []string{extractAssignTarget(trimmed)},
			})
		}
		// DOM-based sources
		if tsFetchSrc.MatchString(trimmed) {
			pf.Nodes = append(pf.Nodes, ParsedNode{
				Kind: KindSource, Symbol: "location", Framework: framework,
				Line: lineno, Text: trimmed,
				TaintedVars: []string{extractAssignTarget(trimmed)},
			})
		}

		// XSS sinks
		if tsInnerHTMLSink.MatchString(trimmed) {
			pf.Nodes = append(pf.Nodes, ParsedNode{
				Kind: KindSink, Symbol: "innerHTML", VulnClass: "xss",
				Framework: framework, Line: lineno, Text: trimmed,
			})
		}
		if tsDangSink.MatchString(trimmed) {
			pf.Nodes = append(pf.Nodes, ParsedNode{
				Kind: KindSink, Symbol: "dangerouslySetInnerHTML", VulnClass: "xss",
				Framework: framework, Line: lineno, Text: trimmed,
			})
		}
		// RCE sinks
		if tsEvalSink.MatchString(trimmed) {
			pf.Nodes = append(pf.Nodes, ParsedNode{
				Kind: KindSink, Symbol: "eval", VulnClass: "rce",
				Framework: framework, Line: lineno, Text: trimmed,
			})
		}
		if tsNodeExecSink.MatchString(trimmed) {
			pf.Nodes = append(pf.Nodes, ParsedNode{
				Kind: KindSink, Symbol: extractCallName(trimmed, tsNodeExecSink),
				VulnClass: "rce", Framework: framework, Line: lineno, Text: trimmed,
			})
		}
		// Path traversal sinks
		if tsFSSink.MatchString(trimmed) {
			pf.Nodes = append(pf.Nodes, ParsedNode{
				Kind: KindSink, Symbol: extractCallName(trimmed, tsFSSink),
				VulnClass: "path_traversal", Framework: framework, Line: lineno, Text: trimmed,
			})
		}
		// SSRF sinks
		if tsFetchSink.MatchString(trimmed) {
			pf.Nodes = append(pf.Nodes, ParsedNode{
				Kind: KindSink, Symbol: "fetch", VulnClass: "ssrf",
				Framework: framework, Line: lineno, Text: trimmed,
			})
		}
		// Sanitizers
		if tsDOMPurify.MatchString(trimmed) {
			pf.Nodes = append(pf.Nodes, ParsedNode{
				Kind: KindSanitizer, Symbol: "DOMPurify.sanitize", VulnClass: "xss",
				Framework: framework, Line: lineno, Text: trimmed,
			})
		}
		if tsEncodeURI.MatchString(trimmed) {
			pf.Nodes = append(pf.Nodes, ParsedNode{
				Kind: KindSanitizer, Symbol: "encodeURIComponent", VulnClass: "xss",
				Framework: framework, Line: lineno, Text: trimmed,
			})
		}
		if tsPreparedStmt.MatchString(trimmed) {
			pf.Nodes = append(pf.Nodes, ParsedNode{
				Kind: KindSanitizer, Symbol: "parameterized_query", VulnClass: "sqli",
				Framework: framework, Line: lineno, Text: trimmed,
			})
		}
	}
}

// ---- Go patterns ----

var (
	goImportRe     = regexp.MustCompile(`"([^"]+)"`)
	goChiSource    = regexp.MustCompile(`chi\.URLParam\s*\(|r\.PathValue\s*\(`)
	goNetHTTPSrc   = regexp.MustCompile(`r\.(URL\.Query\(\)\.Get|FormValue|PostFormValue|Header\.Get|Cookie)\s*\(`)
	goGinSource    = regexp.MustCompile(`c\.(?:Param|Query|PostForm|GetRawData|ShouldBind|Bind)\s*\(`)
	goSQLSink      = regexp.MustCompile(`(?:db|tx|stmt|pool|conn|pgxpool)\.(?:Query|QueryRow|Exec|QueryContext|ExecContext|QueryRowContext|Begin|Send)\s*\(`)
	goRCESink      = regexp.MustCompile(`exec\.Command\s*\(|os\.(?:StartProcess|Create)\s*\(`)
	goFileSink     = regexp.MustCompile(`os\.(?:Open|OpenFile|Create|ReadFile|WriteFile|Remove|Rename)\s*\(`)
	goHTTPSink     = regexp.MustCompile(`http\.(?:Get|Post|Head|Do)\s*\(`)
	goTmplSink     = regexp.MustCompile(`template\.HTML\(|html/template.*unsafe`)
	goHTMLSanitize = regexp.MustCompile(`html\.EscapeString\s*\(|template\.HTMLEscapeString\s*\(`)
	goPgxParam     = regexp.MustCompile(`pgx\.|pgxpool\.|\.QueryRow\s*\(ctx[^,)]*,\s*"[^"]*"`)
	goFuncDef      = regexp.MustCompile(`^func\s+(?:\(\w+\s+\*?\w+\)\s+)?(\w+)\s*\(([^)]*)\)`)
	goHTTPHandler  = regexp.MustCompile(`\(\s*\w+\s+http\.ResponseWriter\s*,\s*\w+\s+\*http\.Request\s*\)`)
	goInBlock      = false
)

func parseGo(pf *ParsedFile, lines []string) {
	framework := ""
	inImportBlock := false

	for i, line := range lines {
		lineno := i + 1
		trimmed := strings.TrimSpace(line)

		// Import detection
		if trimmed == "import (" {
			inImportBlock = true
			continue
		}
		if inImportBlock {
			if trimmed == ")" {
				inImportBlock = false
				continue
			}
			if m := goImportRe.FindStringSubmatch(trimmed); m != nil {
				pkg := m[1]
				pf.Imports = append(pf.Imports, pkg)
				switch {
				case strings.Contains(pkg, "go-chi/chi"):
					framework = "chi"
				case strings.Contains(pkg, "gin-gonic/gin"):
					framework = "gin"
				case strings.Contains(pkg, "jackc/pgx"):
					// pgx detected
				}
			}
			continue
		}
		if strings.HasPrefix(trimmed, `import "`) {
			if m := goImportRe.FindStringSubmatch(trimmed); m != nil {
				pf.Imports = append(pf.Imports, m[1])
			}
		}

		// HTTP sources
		if goChiSource.MatchString(trimmed) {
			pf.Nodes = append(pf.Nodes, ParsedNode{
				Kind: KindSource, Symbol: "chi.URLParam", Framework: "chi",
				Line: lineno, Text: trimmed,
				TaintedVars: []string{extractGoAssignTarget(trimmed)},
			})
		}
		if goNetHTTPSrc.MatchString(trimmed) {
			m := goNetHTTPSrc.FindStringSubmatch(trimmed)
			pf.Nodes = append(pf.Nodes, ParsedNode{
				Kind: KindSource, Symbol: "r." + m[1], Framework: framework,
				Line: lineno, Text: trimmed,
				TaintedVars: []string{extractGoAssignTarget(trimmed)},
			})
		}
		if goGinSource.MatchString(trimmed) {
			m := goGinSource.FindStringSubmatch(trimmed)
			pf.Nodes = append(pf.Nodes, ParsedNode{
				Kind: KindSource, Symbol: "c." + m[1], Framework: "gin",
				Line: lineno, Text: trimmed,
				TaintedVars: []string{extractGoAssignTarget(trimmed)},
			})
		}

		// Function definitions
		if m := goFuncDef.FindStringSubmatch(trimmed); m != nil {
			isHandler := goHTTPHandler.MatchString(trimmed)
			fd := FuncDef{
				Name: m[1], Line: lineno,
				Params: extractGoParams(m[2]),
				IsHandler: isHandler, Framework: framework,
			}
			pf.Functions = append(pf.Functions, fd)
		}

		// SQL sinks
		if goSQLSink.MatchString(trimmed) {
			pf.Nodes = append(pf.Nodes, ParsedNode{
				Kind: KindSink, Symbol: extractCallName(trimmed, goSQLSink),
				VulnClass: "sqli", Framework: framework,
				Line: lineno, Text: trimmed,
			})
		}
		// RCE sinks
		if goRCESink.MatchString(trimmed) {
			pf.Nodes = append(pf.Nodes, ParsedNode{
				Kind: KindSink, Symbol: extractCallName(trimmed, goRCESink),
				VulnClass: "rce", Framework: framework,
				Line: lineno, Text: trimmed,
			})
		}
		// Path traversal sinks
		if goFileSink.MatchString(trimmed) {
			pf.Nodes = append(pf.Nodes, ParsedNode{
				Kind: KindSink, Symbol: extractCallName(trimmed, goFileSink),
				VulnClass: "path_traversal", Framework: framework,
				Line: lineno, Text: trimmed,
			})
		}
		// SSRF sinks
		if goHTTPSink.MatchString(trimmed) {
			pf.Nodes = append(pf.Nodes, ParsedNode{
				Kind: KindSink, Symbol: extractCallName(trimmed, goHTTPSink),
				VulnClass: "ssrf", Framework: framework,
				Line: lineno, Text: trimmed,
			})
		}
		// XSS / template injection
		if goTmplSink.MatchString(trimmed) {
			pf.Nodes = append(pf.Nodes, ParsedNode{
				Kind: KindSink, Symbol: "template.HTML", VulnClass: "xss",
				Framework: framework, Line: lineno, Text: trimmed,
			})
		}
		// Sanitizers
		if goHTMLSanitize.MatchString(trimmed) {
			pf.Nodes = append(pf.Nodes, ParsedNode{
				Kind: KindSanitizer, Symbol: "html.EscapeString", VulnClass: "xss",
				Framework: framework, Line: lineno, Text: trimmed,
			})
		}
		if goPgxParam.MatchString(trimmed) {
			pf.Nodes = append(pf.Nodes, ParsedNode{
				Kind: KindSanitizer, Symbol: "pgx.parameterized", VulnClass: "sqli",
				Framework: framework, Line: lineno, Text: trimmed,
			})
		}
	}
	_ = goInBlock
}

// ---- Helpers ----

func splitLines(content string) []string {
	var lines []string
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines
}

func extractCallName(line string, re *regexp.Regexp) string {
	m := re.FindString(line)
	m = strings.TrimSuffix(m, "(")
	parts := strings.FieldsFunc(m, func(r rune) bool {
		return r == '.' || r == ' ' || r == '\t'
	})
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return m
}

// extractAssignTarget extracts the left-hand side of an assignment (Python/TS style: `x = ...`).
func extractAssignTarget(line string) string {
	line = strings.TrimSpace(line)
	if idx := strings.Index(line, " = "); idx > 0 {
		lhs := strings.TrimSpace(line[:idx])
		// strip type annotation: `x: str = ...` → `x`
		if ci := strings.Index(lhs, ":"); ci > 0 {
			lhs = strings.TrimSpace(lhs[:ci])
		}
		return lhs
	}
	return ""
}

// extractGoAssignTarget extracts the left-hand side of a Go short declaration (`x := ...`).
func extractGoAssignTarget(line string) string {
	line = strings.TrimSpace(line)
	if idx := strings.Index(line, " := "); idx > 0 {
		lhs := strings.TrimSpace(line[:idx])
		// handle multi-return: `x, err := ...` → `x`
		parts := strings.Split(lhs, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}
	return ""
}

func extractPythonParams(paramStr string) []string {
	var out []string
	for _, p := range strings.Split(paramStr, ",") {
		p = strings.TrimSpace(p)
		if p == "" || p == "*" || strings.HasPrefix(p, "**") || strings.HasPrefix(p, "*") {
			continue
		}
		// Strip type annotation and default: `user_id: str = None` → `user_id`
		p = strings.Split(p, ":")[0]
		p = strings.Split(p, "=")[0]
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func extractGoParams(paramStr string) []string {
	var out []string
	for _, p := range strings.Split(paramStr, ",") {
		p = strings.TrimSpace(p)
		parts := strings.Fields(p)
		if len(parts) > 0 {
			out = append(out, parts[0])
		}
	}
	return out
}
