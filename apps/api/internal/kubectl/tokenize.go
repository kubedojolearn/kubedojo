// Package kubectl runs an allowlisted subset of kubectl against the simulated cluster.
//
// Input is never given to a shell. It is split into words by a small tokenizer that understands
// single quotes, double quotes and backslash escapes, and rejects everything a shell would
// interpret (operators, redirections, substitutions, variables, multi-line input) instead of
// trying to sanitize it. Only `kubectl` (or `k`), `help` and `clear` are recognized programs.
package kubectl

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Input limits (simulator-compatibility.md §3).
const (
	MaxInputBytes = 16 << 10
	MaxTokens     = 64
)

// ErrorCode classifies failed commands for the UI (OpenAPI CommandResult.errorCode).
type ErrorCode string

// Error codes.
const (
	CodeUnsupported ErrorCode = "UNSUPPORTED_COMMAND"
	CodeValidation  ErrorCode = "VALIDATION_FAILED"
	CodeNotFound    ErrorCode = "NOT_FOUND"
)

// Failure is a command that could not run. It becomes terminal output, not an API error.
type Failure struct {
	Code    ErrorCode
	Message string // printed to stderr, without a trailing newline
	Exit    int
}

func (f *Failure) Error() string { return f.Message }

func reject(code ErrorCode, format string, args ...any) *Failure {
	return &Failure{Code: code, Message: fmt.Sprintf(format, args...), Exit: 1}
}

func shellOperator(op string) *Failure {
	return reject(CodeUnsupported, "error: %s is shell syntax, and the simulator has no shell. Run one kubectl command at a time.", op)
}

// Tokenize splits a command line into words. It never expands anything: text inside single
// quotes is literal, double quotes allow \" and \\, and a backslash outside quotes escapes the
// next character. Shell operators and substitutions are rejected wherever a shell would act on
// them.
func Tokenize(input string) ([]string, *Failure) {
	if len(input) > MaxInputBytes {
		return nil, reject(CodeValidation, "error: the command is longer than %d KiB", MaxInputBytes>>10)
	}
	if !utf8.ValidString(input) {
		return nil, reject(CodeValidation, "error: the command is not valid UTF-8")
	}
	if strings.ContainsAny(input, "\n\r") {
		return nil, reject(CodeUnsupported, "error: enter one command at a time (multi-line input isn't supported)")
	}
	for _, r := range input {
		if r != '\t' && (unicode.IsControl(r) || r == ' ' || r == ' ') {
			return nil, reject(CodeValidation, "error: the command contains a control character")
		}
	}

	const (
		bare = iota
		single
		double
	)
	var (
		tokens  []string
		cur     strings.Builder
		started bool // a token is open (possibly empty, e.g. "")
		mode    = bare
	)
	flush := func() {
		if started {
			tokens = append(tokens, cur.String())
			cur.Reset()
			started = false
		}
	}
	runes := []rune(input)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		next := rune(0)
		if i+1 < len(runes) {
			next = runes[i+1]
		}
		switch mode {
		case single:
			if r == '\'' {
				mode = bare
			} else {
				cur.WriteRune(r)
			}
			continue
		case double:
			switch {
			case r == '"':
				mode = bare
			case r == '\\' && (next == '"' || next == '\\' || next == '$' || next == '`'):
				cur.WriteRune(next)
				i++
			case r == '`':
				return nil, shellOperator("`...` (command substitution)")
			case r == '$':
				return nil, dollar(next)
			default:
				cur.WriteRune(r)
			}
			continue
		}

		switch r {
		case ' ', '\t':
			flush()
		case '\'':
			mode, started = single, true
		case '"':
			mode, started = double, true
		case '\\':
			if next == 0 {
				return nil, reject(CodeValidation, "error: the command ends with a backslash")
			}
			cur.WriteRune(next)
			started = true
			i++
		case ';':
			return nil, shellOperator("';'")
		case '|':
			if next == '|' {
				return nil, shellOperator("'||'")
			}
			return nil, shellOperator("'|' (a pipe)")
		case '&':
			if next == '&' {
				return nil, shellOperator("'&&'")
			}
			return nil, shellOperator("'&'")
		case '>', '<':
			return nil, shellOperator(fmt.Sprintf("'%c' (redirection)", r))
		case '`':
			return nil, shellOperator("`...` (command substitution)")
		case '$':
			return nil, dollar(next)
		case '(', ')':
			return nil, shellOperator(fmt.Sprintf("'%c'", r))
		default:
			cur.WriteRune(r)
			started = true
		}
	}
	if mode != bare {
		return nil, reject(CodeValidation, "error: unterminated quoted string")
	}
	flush()
	if len(tokens) > MaxTokens {
		return nil, reject(CodeValidation, "error: the command has more than %d words", MaxTokens)
	}
	return tokens, nil
}

func dollar(next rune) *Failure {
	if next == '(' {
		return shellOperator("$(...) (command substitution)")
	}
	return shellOperator("'$' (variables)")
}
