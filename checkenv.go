package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
)

const CHECKENV_VERSION = "v0.0.6"

type showSpec struct {
	loadFrom      map[string]interface{}
	providersFull map[string]interface{}
	providerVars  map[string]map[string]interface{}
}

func parseShowSpec(args []string) *showSpec {
	spec := showSpec{loadFrom: map[string]interface{}{}, providersFull: map[string]interface{}{}, providerVars: map[string]map[string]interface{}{}}
	for _, arg := range args {
		components := strings.Split(arg, "://")
		// TODO(zomglings): Can environment variable names contain the characters "://"? I don't think so.
		// However, it is possible that the provider filter arguments *could* contain those characters. That
		// means that this logic is wrong. We should probably use a different separator.
		providerSpec := components[0]
		spec.loadFrom[providerSpec] = nil
		if len(components) == 1 {
			spec.providersFull[providerSpec] = nil
		} else {
			if _, ok := spec.providerVars[providerSpec]; !ok {
				spec.providerVars[providerSpec] = map[string]interface{}{}
			}
			varSpec := strings.Join(components[1:], "://")
			varNames := strings.Split(varSpec, ",")
			for _, varName := range varNames {
				spec.providerVars[providerSpec][varName] = nil
			}
		}
	}

	return &spec
}

func VariablesFromProviderSpec(providerSpec string) (map[string]string, error) {
	components := strings.Split(providerSpec, "+")
	provider := components[0]
	var providerArgs string
	if len(components) > 1 {
		providerArgs = strings.Join(components[1:], "+")
	}
	plugin, pluginExists := RegisteredPlugins[provider]
	if !pluginExists {
		return map[string]string{}, fmt.Errorf("unregistered provider: %s", provider)
	}
	return plugin.Provider(providerArgs)
}

// Append quotes to value if needed
func AddValueQuotes(val string, showQuotes bool) string {
	if showQuotes {
		return fmt.Sprintf("\"%s\"", val)
	}

	return val
}

// Cut off the path and return only the key after the '/'.
func RemoveKeyPath(key string, removePath bool) string {
	if removePath {
		valSlice := strings.Split(key, "/")
		return valSlice[len(valSlice)-1]
	}

	return key
}

func main() {
	pluginsCommand := "plugins"
	pluginsFlags := flag.NewFlagSet("plugins", flag.ExitOnError)
	pluginsHelp := pluginsFlags.Bool("h", false, "Use this flag if you want help with this command")
	pluginsFlags.BoolVar(pluginsHelp, "help", false, "Use this flag if you want help with this command")

	showCommand := "show"
	showFlags := flag.NewFlagSet("show", flag.ExitOnError)
	showHelp := showFlags.Bool("h", false, "Use this flag if you want help with this command")
	showFlags.BoolVar(showHelp, "help", false, "Use this flag if you want help with this command")
	showExport := showFlags.Bool("export", false, "Use this flag to prepend and \"export \" before every environment variable definition")
	showQuotes := showFlags.Bool("quotes", false, "Use this flag to put value in quotes")
	showRemovePath := showFlags.Bool("remove-path", false, "Use this flag to cut off the path and return only the key after the '/'")
	showRaw := showFlags.Bool("raw", false, "Use this flag to prevent comments output")
	showValue := showFlags.Bool("value", false, "Print value only")
	showName := showFlags.Bool("name", false, "Print name only")

	versionCommand := "version"

	availableCommands := fmt.Sprintf("%s,%s,%s", pluginsCommand, showCommand, versionCommand)

	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Please use one of the subcommands: %s\n", availableCommands)
		os.Exit(2)
	}

	command := os.Args[1]

	switch command {
	case pluginsCommand:
		pluginsFlags.Parse(os.Args[2:])
		if *pluginsHelp {
			fmt.Fprintf(os.Stderr, "Usage: %s %s\nTakes no arguments.\nLists available plugins with a brief description of each one.\n", os.Args[0], os.Args[1])
			os.Exit(2)
		}
		fmt.Println("Available plugins:")
		for name, plugin := range RegisteredPlugins {
			fmt.Printf("%s\n\t%s\n", name, plugin.Help)
		}
	case showCommand:
		showFlags.Parse(os.Args[2:])
		if *showHelp || showFlags.NArg() == 0 {
			fmt.Fprintf(os.Stderr, "Usage: %s %s [<provider_name>[+<provider_args>] ...] [<provider_name>[+<provider_args>]://<var_name_1>,<var_name_2>,...,<var_name_n> ...]\nShows the environment variables defined by the given providers.\n", os.Args[0], os.Args[1])
			os.Exit(2)
		}

		if *showName && *showValue {
			fmt.Fprintf(os.Stderr, "You can't use both -name and -value flags at the same time.\n")
			os.Exit(1)
		}

		spec := parseShowSpec(showFlags.Args())
		providedVars := make(map[string]map[string]string)
		for providerSpec := range spec.loadFrom {
			vars, providerErr := VariablesFromProviderSpec(providerSpec)
			if providerErr != nil {
				log.Fatal(providerErr.Error())
			}
			providedVars[providerSpec] = vars
		}

		exportPrefix := ""
		if *showExport {
			exportPrefix = "export "
		}

		for providerSpec := range spec.providersFull {
			if !*showRaw {
				fmt.Printf("# Generated with %s - all variables:\n", providerSpec)
			}
			for k, v := range providedVars[providerSpec] {
				if !*showValue && !*showName {
					fmt.Printf("%s%s=%s\n", exportPrefix, RemoveKeyPath(k, *showRemovePath), AddValueQuotes(v, *showQuotes))
				} else if *showValue {
					fmt.Printf("%s\n", AddValueQuotes(v, *showQuotes))
				} else if *showName {
					fmt.Printf("%s\n", RemoveKeyPath(k, *showRemovePath))
				}
			}
		}
		for providerSpec, queriedVars := range spec.providerVars {
			if !*showRaw {
				fmt.Printf("# Generated with %s - specific variables:\n", providerSpec)
			}
			definedVars := providedVars[providerSpec]
			for k := range queriedVars {
				v, ok := definedVars[k]
				if !ok {
					fmt.Printf("# UNDEFINED: %s\n", k)
				} else {
					if !*showValue {
						fmt.Printf("%s%s=%s\n", exportPrefix, k, AddValueQuotes(v, *showQuotes))
					} else {
						fmt.Printf("%s\n", AddValueQuotes(v, *showQuotes))
					}
				}
			}
		}
	case versionCommand:
		pluginsFlags.Parse(os.Args[2:])
		if *pluginsHelp {
			fmt.Fprintf(os.Stderr, "Usage: %s %s\nShows version of checkenv.\n", os.Args[0], os.Args[1])
			os.Exit(2)
		}
		fmt.Println(CHECKENV_VERSION)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s. Please use one of the subcommands: %s.\n", command, availableCommands)
	}
}
