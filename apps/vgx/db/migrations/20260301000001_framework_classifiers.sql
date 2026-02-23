-- migrate:up

-- framework_classifiers: built-in source/sink/sanitizer definitions per framework
-- These seed the SPG classification pipeline at init time.
CREATE TABLE framework_classifiers (
    id          BIGSERIAL   PRIMARY KEY,
    framework   TEXT        NOT NULL,   -- django|fastapi|express|nextjs|gin|spring
    language    TEXT        NOT NULL,   -- python|typescript|go|java
    node_type   TEXT        NOT NULL CHECK (node_type IN ('SourceNode', 'SinkNode', 'SanitizerNode')),
    symbol      TEXT        NOT NULL,   -- function/method/attribute pattern
    vuln_class  TEXT,                   -- which vuln class this sink enables (sinks only)
    description TEXT,
    is_builtin  BOOL        NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_classifiers_framework ON framework_classifiers(framework, language);
CREATE INDEX idx_classifiers_type      ON framework_classifiers(node_type);

-- Seed: Python / FastAPI / Django sources
INSERT INTO framework_classifiers (framework, language, node_type, symbol, description) VALUES
    ('fastapi',  'python', 'SourceNode', 'Request.body',         'FastAPI raw request body'),
    ('fastapi',  'python', 'SourceNode', 'Request.query_params', 'FastAPI query string params'),
    ('fastapi',  'python', 'SourceNode', 'Request.path_params',  'FastAPI URL path params'),
    ('fastapi',  'python', 'SourceNode', 'Request.headers',      'FastAPI request headers'),
    ('fastapi',  'python', 'SourceNode', 'Request.cookies',      'FastAPI cookies'),
    ('django',   'python', 'SourceNode', 'HttpRequest.POST',     'Django POST data'),
    ('django',   'python', 'SourceNode', 'HttpRequest.GET',      'Django GET query params'),
    ('django',   'python', 'SourceNode', 'HttpRequest.FILES',    'Django uploaded files'),
    ('django',   'python', 'SourceNode', 'HttpRequest.COOKIES',  'Django cookies'),
    ('django',   'python', 'SourceNode', 'HttpRequest.META',     'Django request metadata');

-- Seed: Python sinks
INSERT INTO framework_classifiers (framework, language, node_type, symbol, vuln_class, description) VALUES
    ('python',   'python', 'SinkNode', 'cursor.execute',       'sqli',          'Raw SQL execution via DB cursor'),
    ('python',   'python', 'SinkNode', 'os.system',            'rce',           'Shell command execution'),
    ('python',   'python', 'SinkNode', 'subprocess.run',       'rce',           'Subprocess execution'),
    ('python',   'python', 'SinkNode', 'subprocess.call',      'rce',           'Subprocess execution'),
    ('python',   'python', 'SinkNode', 'eval',                 'rce',           'Dynamic code evaluation'),
    ('python',   'python', 'SinkNode', 'exec',                 'rce',           'Dynamic code execution'),
    ('python',   'python', 'SinkNode', 'open',                 'path_traversal','File open — path may be attacker-controlled'),
    ('python',   'python', 'SinkNode', 'pickle.loads',         'deserialization','Unsafe deserialization'),
    ('python',   'python', 'SinkNode', 'yaml.load',            'deserialization','Unsafe YAML deserialization'),
    ('python',   'python', 'SinkNode', 'requests.get',         'ssrf',          'Outbound HTTP — URL may be attacker-controlled'),
    ('python',   'python', 'SinkNode', 'httpx.get',            'ssrf',          'Outbound HTTP — URL may be attacker-controlled');

-- Seed: TypeScript / Express / Next.js sources
INSERT INTO framework_classifiers (framework, language, node_type, symbol, description) VALUES
    ('express',  'typescript', 'SourceNode', 'req.body',         'Express request body'),
    ('express',  'typescript', 'SourceNode', 'req.query',        'Express query string'),
    ('express',  'typescript', 'SourceNode', 'req.params',       'Express URL params'),
    ('express',  'typescript', 'SourceNode', 'req.headers',      'Express request headers'),
    ('express',  'typescript', 'SourceNode', 'req.cookies',      'Express cookies'),
    ('nextjs',   'typescript', 'SourceNode', 'request.json()',   'Next.js request JSON body'),
    ('nextjs',   'typescript', 'SourceNode', 'searchParams',     'Next.js URL search params'),
    ('nextjs',   'typescript', 'SourceNode', 'params',           'Next.js dynamic route params');

-- Seed: TypeScript sinks
INSERT INTO framework_classifiers (framework, language, node_type, symbol, vuln_class, description) VALUES
    ('typescript','typescript','SinkNode', 'innerHTML',          'xss',          'Direct HTML injection'),
    ('typescript','typescript','SinkNode', 'dangerouslySetInnerHTML','xss',      'React unsafe HTML injection'),
    ('typescript','typescript','SinkNode', 'eval',               'rce',          'Dynamic code evaluation'),
    ('typescript','typescript','SinkNode', 'document.write',     'xss',          'Document write injection'),
    ('node',     'typescript', 'SinkNode', 'child_process.exec', 'rce',          'Shell command execution'),
    ('node',     'typescript', 'SinkNode', 'child_process.spawn','rce',          'Process spawning'),
    ('node',     'typescript', 'SinkNode', 'fs.readFile',        'path_traversal','File read — path may be attacker-controlled'),
    ('node',     'typescript', 'SinkNode', 'fs.writeFile',       'path_traversal','File write — path may be attacker-controlled'),
    ('node',     'typescript', 'SinkNode', 'fetch',              'ssrf',         'Outbound fetch — URL may be attacker-controlled');

-- Seed: Go / Gin sources
INSERT INTO framework_classifiers (framework, language, node_type, symbol, description) VALUES
    ('gin',      'go', 'SourceNode', 'c.Param',        'Gin URL path param'),
    ('gin',      'go', 'SourceNode', 'c.Query',        'Gin query string param'),
    ('gin',      'go', 'SourceNode', 'c.PostForm',     'Gin POST form field'),
    ('gin',      'go', 'SourceNode', 'c.GetRawData',   'Gin raw request body'),
    ('chi',      'go', 'SourceNode', 'chi.URLParam',   'Chi URL path param'),
    ('net/http', 'go', 'SourceNode', 'r.URL.Query',    'net/http query params'),
    ('net/http', 'go', 'SourceNode', 'r.FormValue',    'net/http form value'),
    ('net/http', 'go', 'SourceNode', 'r.PostFormValue','net/http POST form value');

-- Seed: Go sinks
INSERT INTO framework_classifiers (framework, language, node_type, symbol, vuln_class, description) VALUES
    ('go',       'go', 'SinkNode', 'db.Query',          'sqli',          'Raw SQL query via database/sql'),
    ('go',       'go', 'SinkNode', 'db.Exec',           'sqli',          'Raw SQL exec via database/sql'),
    ('go',       'go', 'SinkNode', 'os.exec.Command',   'rce',           'OS command execution'),
    ('go',       'go', 'SinkNode', 'exec.Command',      'rce',           'exec.Command shell execution'),
    ('go',       'go', 'SinkNode', 'os.Open',           'path_traversal','File open — path may be attacker-controlled'),
    ('go',       'go', 'SinkNode', 'http.Get',          'ssrf',          'Outbound HTTP — URL may be attacker-controlled'),
    ('go',       'go', 'SinkNode', 'template.HTML',     'xss',           'Unsafe HTML template injection');

-- Seed: common sanitizers (cross-language)
INSERT INTO framework_classifiers (framework, language, node_type, symbol, description) VALUES
    ('python',      'python',     'SanitizerNode', 'html.escape',           'HTML entity escaping'),
    ('django',      'python',     'SanitizerNode', 'mark_safe',             'Django explicit safe-mark (intentional bypass)'),
    ('django',      'python',     'SanitizerNode', 'escape',                'Django auto HTML escaping'),
    ('sqlalchemy',  'python',     'SanitizerNode', 'text().bindparams',     'SQLAlchemy parameterized query'),
    ('typescript',  'typescript', 'SanitizerNode', 'DOMPurify.sanitize',    'DOMPurify HTML sanitizer'),
    ('typescript',  'typescript', 'SanitizerNode', 'encodeURIComponent',    'URL encoding'),
    ('typescript',  'typescript', 'SanitizerNode', 'escape',                'HTML entity escaping'),
    ('go',          'go',         'SanitizerNode', 'html.EscapeString',     'Go HTML escaping'),
    ('go',          'go',         'SanitizerNode', 'pgx.NamedArgs',         'pgx parameterized query'),
    ('go',          'go',         'SanitizerNode', 'pgx.QueryRow',          'pgx parameterized row query');

-- migrate:down
DROP TABLE IF EXISTS framework_classifiers;
