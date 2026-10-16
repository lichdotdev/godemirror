package godemirror

import "github.com/lichdotdev/godemirror/tags"

// The builtin sets tag a language's builtins by their text, with
// StyleRule.Names, where the grammar parses them as names like any other.
// Shadowing is not seen: a local variable named len is tagged a builtin too.

// goBuiltinTypes and goBuiltinFuncs are the predeclared types and
// functions of the Go specification (go.dev/ref/spec#Predeclared_identifiers).
var (
	goBuiltinTypes = []string{
		"any", "bool", "byte", "comparable", "complex64", "complex128", "error", "float32", "float64",
		"int", "int8", "int16", "int32", "int64", "rune", "string",
		"uint", "uint8", "uint16", "uint32", "uint64", "uintptr",
	}
	goBuiltinFuncs = []string{
		"append", "cap", "clear", "close", "complex", "copy", "delete", "imag", "len", "make",
		"max", "min", "new", "panic", "print", "println", "real", "recover",
	}
)

// GoBuiltinTags tags Go's predeclared identifiers: a type is
// standard(typeName), a call of a builtin function
// function(standard(variableName)), a conversion to a builtin type, as in
// int(x), standard(typeName), and iota standard(variableName). true, false
// and nil are already literals in @lezer/go, and make before a type is a
// keyword RicherGoTags tags.
func GoBuiltinTags() StyleTags {
	return StyleTags{Language: "go", Rules: []StyleRule{
		{Selector: "TypeName", Names: goBuiltinTypes, Tags: []*tags.Tag{tags.Standard(tags.TypeName)}},
		{Selector: "CallExpr/VariableName", Names: goBuiltinFuncs, Tags: []*tags.Tag{tags.Function(tags.Standard(tags.VariableName))}},
		{Selector: "CallExpr/VariableName", Names: goBuiltinTypes, Tags: []*tags.Tag{tags.Standard(tags.TypeName)}},
		{Selector: "VariableName", Names: []string{"iota"}, Tags: []*tags.Tag{tags.Standard(tags.VariableName)}},
	}}
}

// pythonBuiltins are the built-in functions and types of Python 3.13's
// library reference (docs.python.org/3.13/library/functions.html), but print
// and type, which @lezer/python parses as keywords.
var pythonBuiltins = []string{
	"abs", "aiter", "all", "anext", "any", "ascii", "bin", "bool", "breakpoint", "bytearray", "bytes",
	"callable", "chr", "classmethod", "compile", "complex", "delattr", "dict", "dir", "divmod",
	"enumerate", "eval", "exec", "filter", "float", "format", "frozenset", "getattr", "globals",
	"hasattr", "hash", "help", "hex", "id", "input", "int", "isinstance", "issubclass", "iter", "len",
	"list", "locals", "map", "max", "memoryview", "min", "next", "object", "oct", "open", "ord", "pow",
	"property", "range", "repr", "reversed", "round", "set", "setattr", "slice", "sorted",
	"staticmethod", "str", "sum", "super", "tuple", "vars", "zip", "__import__",
}

// PythonBuiltinTags tags Python's built-in functions and types: called,
// function(standard(variableName)), and standard(variableName) anywhere
// else (isinstance(x, int)).
func PythonBuiltinTags() StyleTags {
	return StyleTags{Language: "python", Rules: []StyleRule{
		{Selector: "CallExpression/VariableName", Names: pythonBuiltins, Tags: []*tags.Tag{tags.Function(tags.Standard(tags.VariableName))}},
		{Selector: "VariableName", Names: pythonBuiltins, Tags: []*tags.Tag{tags.Standard(tags.VariableName)}},
	}}
}

// The global object's properties of ECMAScript 2025 (tc39.es/ecma262/2025,
// section 19), split by how they are used. Host globals (console, window,
// process) are not in it: they differ by host.
var (
	jsGlobalValues = []string{"globalThis", "Infinity", "NaN", "undefined", "Atomics", "JSON", "Math", "Reflect", "Intl"}
	jsGlobalFuncs  = []string{
		"eval", "isFinite", "isNaN", "parseFloat", "parseInt",
		"decodeURI", "decodeURIComponent", "encodeURI", "encodeURIComponent",
	}
	jsGlobalClasses = []string{
		"AggregateError", "Array", "ArrayBuffer", "BigInt", "BigInt64Array", "BigUint64Array", "Boolean",
		"DataView", "Date", "Error", "EvalError", "FinalizationRegistry", "Float32Array", "Float64Array",
		"Function", "Int8Array", "Int16Array", "Int32Array", "Iterator", "Map", "Number", "Object",
		"Promise", "Proxy", "RangeError", "ReferenceError", "RegExp", "Set", "SharedArrayBuffer",
		"String", "Symbol", "SyntaxError", "TypeError", "Uint8Array", "Uint8ClampedArray",
		"Uint16Array", "Uint32Array", "URIError", "WeakMap", "WeakRef", "WeakSet",
	}
	// tsBuiltinTypes are TypeScript's primitive types and top and bottom
	// types, which @lezer/javascript parses as type names.
	tsBuiltinTypes = []string{"any", "bigint", "boolean", "never", "number", "object", "string", "symbol", "undefined", "unknown"}
)

// JavaScriptBuiltinTags tags JavaScript's and TypeScript's builtins: a
// global constructor is standard(className), a global function called
// function(standard(variableName)), any other global standard(variableName),
// and a TypeScript primitive type standard(typeName).
func JavaScriptBuiltinTags() StyleTags {
	globals := append(append(append([]string(nil), jsGlobalValues...), jsGlobalFuncs...), jsGlobalClasses...)
	return StyleTags{Language: "javascript", Rules: []StyleRule{
		{Selector: "NewExpression/VariableName", Names: jsGlobalClasses, Tags: []*tags.Tag{tags.Standard(tags.ClassName)}},
		{Selector: "CallExpression/VariableName", Names: jsGlobalFuncs, Tags: []*tags.Tag{tags.Function(tags.Standard(tags.VariableName))}},
		{Selector: "VariableName", Names: globals, Tags: []*tags.Tag{tags.Standard(tags.VariableName)}},
		{Selector: "TypeName", Names: tsBuiltinTypes, Tags: []*tags.Tag{tags.Standard(tags.TypeName)}},
	}}
}

// rustPrimitiveTypes are the primitive types of The Rust Reference
// (doc.rust-lang.org/reference/types.html), the stable ones.
var rustPrimitiveTypes = []string{
	"bool", "char", "str", "f32", "f64",
	"i8", "i16", "i32", "i64", "i128", "isize", "u8", "u16", "u32", "u64", "u128", "usize",
}

// RustBuiltinTags tags Rust's primitive types standard(typeName). The
// prelude (Vec, Option, Some) is ordinary library code and stays as
// @lezer/rust tags it, as does a primitive type used as a path (i32::MAX),
// which the grammar parses as a namespace.
func RustBuiltinTags() StyleTags {
	return StyleTags{Language: "rust", Rules: []StyleRule{
		{Selector: "TypeIdentifier", Names: rustPrimitiveTypes, Tags: []*tags.Tag{tags.Standard(tags.TypeName)}},
	}}
}
