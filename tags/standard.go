package tags

// The standard tags, with @lezer/highlight's names and parents.
var (
	// Comment is a comment.
	Comment = define("comment", nil)
	// LineComment is a line comment.
	LineComment = define("lineComment", Comment)
	// BlockComment is a block comment.
	BlockComment = define("blockComment", Comment)
	// DocComment is a documentation comment.
	DocComment = define("docComment", Comment)

	// Name is any kind of identifier.
	Name = define("name", nil)
	// VariableName is the name of a variable.
	VariableName = define("variableName", Name)
	// TypeName is a type name.
	TypeName = define("typeName", Name)
	// TagName is a tag name, a subtag of TypeName.
	TagName = define("tagName", TypeName)
	// PropertyName is a property or field name.
	PropertyName = define("propertyName", Name)
	// AttributeName is an attribute name, a subtag of PropertyName.
	AttributeName = define("attributeName", PropertyName)
	// ClassName is the name of a class.
	ClassName = define("className", Name)
	// LabelName is a label name.
	LabelName = define("labelName", Name)
	// Namespace is a namespace name.
	Namespace = define("namespace", Name)
	// MacroName is the name of a macro.
	MacroName = define("macroName", Name)

	// Literal is a literal value.
	Literal = define("literal", nil)
	// String is a string literal.
	String = define("string", Literal)
	// DocString is a documentation string.
	DocString = define("docString", String)
	// Character is a character literal, a subtag of String.
	Character = define("character", String)
	// AttributeValue is an attribute value, a subtag of String.
	AttributeValue = define("attributeValue", String)
	// Number is a number literal.
	Number = define("number", Literal)
	// Integer is an integer literal.
	Integer = define("integer", Number)
	// Float is a floating-point literal.
	Float = define("float", Number)
	// Bool is a boolean literal.
	Bool = define("bool", Literal)
	// Regexp is a regular expression literal.
	Regexp = define("regexp", Literal)
	// Escape is an escape, such as a backslash escape in a string.
	Escape = define("escape", Literal)
	// Color is a color literal.
	Color = define("color", Literal)
	// URL is a URL literal.
	URL = define("url", Literal)

	// Keyword is a language keyword.
	Keyword = define("keyword", nil)
	// Self is the keyword for the self or this object.
	Self = define("self", Keyword)
	// Null is the keyword for null.
	Null = define("null", Keyword)
	// Atom is a keyword denoting some atomic value.
	Atom = define("atom", Keyword)
	// Unit is a keyword that represents a unit.
	Unit = define("unit", Keyword)
	// Modifier is a modifier keyword.
	Modifier = define("modifier", Keyword)
	// OperatorKeyword is a keyword that acts as an operator.
	OperatorKeyword = define("operatorKeyword", Keyword)
	// ControlKeyword is a control-flow keyword.
	ControlKeyword = define("controlKeyword", Keyword)
	// DefinitionKeyword is a keyword that defines something.
	DefinitionKeyword = define("definitionKeyword", Keyword)
	// ModuleKeyword is a keyword about defining or using modules.
	ModuleKeyword = define("moduleKeyword", Keyword)

	// Operator is an operator.
	Operator = define("operator", nil)
	// DerefOperator is an operator that dereferences something.
	DerefOperator = define("derefOperator", Operator)
	// ArithmeticOperator is an arithmetic operator.
	ArithmeticOperator = define("arithmeticOperator", Operator)
	// LogicOperator is a logical operator.
	LogicOperator = define("logicOperator", Operator)
	// BitwiseOperator is a bit operator.
	BitwiseOperator = define("bitwiseOperator", Operator)
	// CompareOperator is a comparison operator.
	CompareOperator = define("compareOperator", Operator)
	// UpdateOperator is an operator that updates its operand.
	UpdateOperator = define("updateOperator", Operator)
	// DefinitionOperator is an operator that defines something.
	DefinitionOperator = define("definitionOperator", Operator)
	// TypeOperator is a type-related operator.
	TypeOperator = define("typeOperator", Operator)
	// ControlOperator is a control-flow operator.
	ControlOperator = define("controlOperator", Operator)

	// Punctuation is program or markup punctuation.
	Punctuation = define("punctuation", nil)
	// Separator is punctuation that separates things.
	Separator = define("separator", Punctuation)
	// Bracket is bracket-style punctuation.
	Bracket = define("bracket", Punctuation)
	// AngleBracket is angle brackets, usually < and >.
	AngleBracket = define("angleBracket", Bracket)
	// SquareBracket is square brackets, usually [ and ].
	SquareBracket = define("squareBracket", Bracket)
	// Paren is parentheses.
	Paren = define("paren", Bracket)
	// Brace is braces.
	Brace = define("brace", Bracket)

	// Content is content, such as plain text in XML or markup documents.
	Content = define("content", nil)
	// Heading is content that represents a heading.
	Heading = define("heading", Content)
	// Heading1 is a level 1 heading.
	Heading1 = define("heading1", Heading)
	// Heading2 is a level 2 heading.
	Heading2 = define("heading2", Heading)
	// Heading3 is a level 3 heading.
	Heading3 = define("heading3", Heading)
	// Heading4 is a level 4 heading.
	Heading4 = define("heading4", Heading)
	// Heading5 is a level 5 heading.
	Heading5 = define("heading5", Heading)
	// Heading6 is a level 6 heading.
	Heading6 = define("heading6", Heading)
	// ContentSeparator is a prose separator, such as a horizontal rule.
	ContentSeparator = define("contentSeparator", Content)
	// List is content that represents a list.
	List = define("list", Content)
	// Quote is content that represents a quote.
	Quote = define("quote", Content)
	// Emphasis is emphasized content.
	Emphasis = define("emphasis", Content)
	// Strong is content styled strong.
	Strong = define("strong", Content)
	// Link is content that is part of a link.
	Link = define("link", Content)
	// Monospace is content styled as code or monospace.
	Monospace = define("monospace", Content)
	// Strikethrough is content with a strike-through style.
	Strikethrough = define("strikethrough", Content)

	// Inserted is inserted text in a change-tracking format.
	Inserted = define("inserted", nil)
	// Deleted is deleted text.
	Deleted = define("deleted", nil)
	// Changed is changed text.
	Changed = define("changed", nil)

	// Invalid is an invalid or unsyntactic element.
	Invalid = define("invalid", nil)

	// Meta is metadata or a meta-instruction.
	Meta = define("meta", nil)
	// DocumentMeta is metadata that applies to the entire document.
	DocumentMeta = define("documentMeta", Meta)
	// Annotation is metadata that annotates a given syntactic element.
	Annotation = define("annotation", Meta)
	// ProcessingInstruction is a processing instruction or preprocessor
	// directive.
	ProcessingInstruction = define("processingInstruction", Meta)
)

// The standard modifiers. Each returns a subtag of the tag it is given,
// falling back to it; applying several, in any order, gives one tag that
// falls back to each smaller set of them.
var (
	// Definition marks an element being defined, mostly for names.
	Definition = defineModifier("definition")
	// Constant marks something constant, mostly a variable name.
	Constant = defineModifier("constant")
	// Function marks a variable or property name called or defined as a
	// function.
	Function = defineModifier("function")
	// Standard marks a name from the language's standard environment.
	Standard = defineModifier("standard")
	// Local marks a name local to some scope.
	Local = defineModifier("local")
	// Special is a generic variant, for language-specific alternatives of
	// a common tag; themes usually style Special(String) and
	// Special(VariableName).
	Special = defineModifier("special")
)
