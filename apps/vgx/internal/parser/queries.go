package parser

// Tree-sitter query strings (.scm syntax) for each language.
// Each query captures security-relevant AST nodes using named captures (@name).

// ---- Python ----

// pyQFuncDef captures all function and async function definitions.
const pyQFuncDef = `
(function_definition
  name: (identifier) @func.name
  parameters: (parameters) @func.params)

(decorated_definition
  (function_definition
    name: (identifier) @func.name
    parameters: (parameters) @func.params))
`

// pyQDecorator captures route decorators (FastAPI, Flask, Blueprint).
const pyQDecorator = `
(decorator
  (call
    function: [
      (attribute object: (identifier) @deco.obj attribute: (identifier) @deco.method)
      (identifier) @deco.name
    ])) @decorator
`

// pyQCall captures all function calls (for call-graph edges).
const pyQCall = `
(call
  function: [
    (identifier) @callee
    (attribute attribute: (identifier) @callee)
  ]
  arguments: (argument_list) @args)
`

// pyQSource captures HTTP request parameter access patterns.
const pyQSource = `
(attribute
  object: (identifier) @req
  attribute: (identifier) @attr) @source
`

// pyQSink captures calls to dangerous functions (SQL, RCE, file, deserialization, SSRF).
const pyQSink = `
(call
  function: (attribute
    object: (identifier) @obj
    attribute: (identifier) @method)
  arguments: (argument_list) @args) @sink
`

// pyQSimpleSink captures module-level dangerous calls (eval, exec, os.system).
const pyQSimpleSink = `
(call
  function: (identifier) @callee
  arguments: (argument_list) @args) @call
`

// pyQImport captures import statements.
const pyQImport = `
(import_statement
  name: [(dotted_name) @pkg
         (aliased_import name: (dotted_name) @pkg)])

(import_from_statement
  module_name: (dotted_name) @pkg)
`

// pyQAssign captures variable assignments (for taint tracking).
const pyQAssign = `
(assignment
  left: (identifier) @lhs
  right: (_) @rhs) @assign
`

// ---- TypeScript / JavaScript ----

// tsQFuncDef captures function and arrow function definitions.
const tsQFuncDef = `
(function_declaration
  name: (identifier) @func.name
  parameters: (formal_parameters) @func.params)

(method_definition
  name: (property_identifier) @func.name
  parameters: (formal_parameters) @func.params)

(lexical_declaration
  (variable_declarator
    name: (identifier) @func.name
    value: (arrow_function
      parameters: (formal_parameters) @func.params)))
`

// tsQCall captures all function/method calls.
const tsQCall = `
(call_expression
  function: [
    (identifier) @callee
    (member_expression property: (property_identifier) @callee)
  ]
  arguments: (arguments) @args)
`

// tsQSource captures request parameter access (Express, Next.js, DOM).
const tsQSource = `
(member_expression
  object: (identifier) @req
  property: (property_identifier) @attr) @source
`

// tsQImport captures import statements and require() calls.
const tsQImport = `
(import_statement
  source: (string (string_fragment) @pkg))

(call_expression
  function: (identifier) @require
  arguments: (arguments (string (string_fragment) @pkg))
  (#eq? @require "require"))
`

// tsQAssign captures variable declarations and assignments.
const tsQAssign = `
(lexical_declaration
  (variable_declarator
    name: (identifier) @lhs
    value: (_) @rhs))

(assignment_expression
  left: (identifier) @lhs
  right: (_) @rhs)
`

// ---- Go ----

// goQFuncDef captures function and method definitions.
const goQFuncDef = `
(function_declaration
  name: (identifier) @func.name
  parameters: (parameter_list) @func.params)

(method_declaration
  name: (field_identifier) @func.name
  parameters: (parameter_list) @func.params)
`

// goQCall captures all function/method call expressions.
const goQCall = `
(call_expression
  function: [
    (identifier) @callee
    (selector_expression field: (field_identifier) @callee)
  ]
  arguments: (argument_list) @args)
`

// goQSource captures HTTP request parameter reads (chi, net/http, gin).
// Uses (_) for operand to also match chained calls like r.URL.Query().Get(...).
const goQSource = `
(call_expression
  function: (selector_expression
    operand: (_) @recv
    field: (field_identifier) @method)
  arguments: (argument_list) @args) @source
`

// goQImport captures import declarations.
const goQImport = `
(import_spec path: (interpreted_string_literal) @pkg)
`

// goQAssign captures short variable declarations.
const goQAssign = `
(short_var_declaration
  left: (expression_list (identifier) @lhs)
  right: (expression_list (_) @rhs))
`
