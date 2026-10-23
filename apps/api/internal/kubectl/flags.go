package kubectl

import (
	"fmt"
	"strconv"
	"strings"
)

// flagDef declares one flag a command accepts.
type flagDef struct {
	Long  string
	Short string
	Value bool // takes a value; otherwise boolean
}

// globalFlags are accepted by every command.
var globalFlags = []flagDef{
	{Long: "namespace", Short: "n", Value: true},
	{Long: "all-namespaces", Short: "A"},
	{Long: "help", Short: "h"},
}

// knownUnsupported are real kubectl flags the simulator doesn't implement, with what to do instead.
var knownUnsupported = map[string]string{
	"watch":                    "watching isn't needed: the simulated cluster is fully reconciled after every command",
	"w":                        "watching isn't needed: the simulated cluster is fully reconciled after every command",
	"watch-only":               "watching isn't needed: the simulated cluster is fully reconciled after every command",
	"follow":                   "following isn't supported; run the command again to see new output",
	"context":                  "the simulator has a single context",
	"kubeconfig":               "the simulator has a single built-in kubeconfig",
	"server":                   "the simulator has no API server address to change",
	"s":                        "the simulator has no API server address to change",
	"token":                    "the simulator needs no credentials",
	"as":                       "impersonation isn't simulated",
	"user":                     "the simulator has a single user",
	"cluster":                  "the simulator has a single cluster",
	"v":                        "verbose API logging isn't simulated",
	"field-selector":           "use -l/--selector instead",
	"raw":                      "raw API requests aren't simulated",
	"chunk-size":               "pagination isn't needed in the simulator",
	"server-print":             "output is always formatted by the simulator",
	"insecure-skip-tls-verify": "the simulator has no TLS connection",
}

type flags struct {
	values map[string][]string // by long name
	order  []string            // long names in the order given
}

func (f flags) str(name string) string {
	v := f.values[name]
	if len(v) == 0 {
		return ""
	}
	return v[len(v)-1]
}

func (f flags) boolean(name string) bool {
	v := f.values[name]
	return len(v) > 0 && v[len(v)-1] == "true"
}

// parseFlags separates positional arguments from flags. cmd names the command for messages
// ("get", "rollout status").
func parseFlags(args []string, cmd string, defs []flagDef) ([]string, flags, *Failure) {
	all := append(append([]flagDef{}, globalFlags...), defs...)
	byLong, byShort := map[string]flagDef{}, map[string]flagDef{}
	for _, d := range all {
		byLong[d.Long] = d
		if d.Short != "" {
			byShort[d.Short] = d
		}
	}
	f := flags{values: map[string][]string{}}
	set := func(d flagDef, v string) {
		f.values[d.Long] = append(f.values[d.Long], v)
		f.order = append(f.order, d.Long)
	}
	usage := fmt.Sprintf("\nSee 'kubectl %s --help' for usage.", cmd)
	var positional []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--":
			positional = append(positional, args[i+1:]...)
			return positional, f, nil
		case strings.HasPrefix(a, "--") && len(a) > 2:
			name, value, hasValue := strings.Cut(a[2:], "=")
			d, ok := byLong[name]
			if !ok {
				return nil, f, unknownFlag("--"+name, name, "--"+name, usage)
			}
			switch {
			case d.Value && hasValue:
				set(d, value)
			case d.Value:
				if i+1 >= len(args) {
					return nil, f, reject(CodeValidation, "error: flag needs an argument: --%s%s", name, usage)
				}
				i++
				set(d, args[i])
			case hasValue:
				b, err := strconv.ParseBool(value)
				if err != nil {
					return nil, f, reject(CodeValidation, "error: invalid argument %q for \"--%s\" flag: strconv.ParseBool: parsing %q: invalid syntax%s", value, name, value, usage)
				}
				set(d, strconv.FormatBool(b))
			default:
				set(d, "true")
			}
		case strings.HasPrefix(a, "-") && len(a) > 1:
			short := a[1:2]
			rest := a[2:]
			d, ok := byShort[short]
			if !ok {
				return nil, f, unknownFlag("-"+short, short, fmt.Sprintf("'%s' in %s", short, a), usage)
			}
			switch {
			case d.Value && rest != "":
				set(d, strings.TrimPrefix(rest, "="))
			case d.Value:
				if i+1 >= len(args) {
					return nil, f, reject(CodeValidation, "error: flag needs an argument: '%s' in -%s%s", short, short, usage)
				}
				i++
				set(d, args[i])
			case rest != "":
				return nil, f, reject(CodeUnsupported, "error: combined short flags like %s aren't supported in the simulator; write them separately%s", a, usage)
			default:
				set(d, "true")
			}
		default:
			positional = append(positional, a)
		}
	}
	return positional, f, nil
}

// unknownFlag reports a flag the command doesn't accept. flag is "--watch" or "-w"; shown is
// how kubectl quotes it in "unknown flag" errors.
func unknownFlag(flag, name, shown, usage string) *Failure {
	if why, ok := knownUnsupported[name]; ok {
		return reject(CodeUnsupported, "error: %s isn't supported in the simulator: %s", flag, why)
	}
	if strings.HasPrefix(flag, "--") {
		return reject(CodeUnsupported, "error: unknown flag: %s%s", shown, usage)
	}
	return reject(CodeUnsupported, "error: unknown shorthand flag: %s%s", shown, usage)
}
