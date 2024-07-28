package components

type App struct {
	Version string
	Namespace
	Subcommands []Namespace
}

func CreateApp(name, version, description string, commands []Command) App {
	return App{
		Version: version,
		Namespace: Namespace{
			Name:        name,
			Description: description,
			Commands:    commands,
		},
	}
}

func CreateEmbeddedApp(name string, commands []Command, namespaces ...Namespace) App {
	app := App{
		Namespace: Namespace{
			Name:     name,
			Commands: commands,
		},
	}
	app.Subcommands = append(app.Subcommands, namespaces...)
	return app
}

type Namespace struct {
	Name        string
	Description string
	Hidden 		bool
	Category    string
	Commands    []Command
}

type Command struct {
	Name            string
	Description     string
	Category        string
	Aliases         []string
	UsageOptions    *UsageOptions
	Arguments       []Argument
	Flags           []Flag
	EnvVars         []EnvVar
	Action          ActionFunc
	SkipFlagParsing bool
	Hidden          bool
}

type UsageOptions struct {
	// Special cases, each of these will be created as command usage option and the value appended as suffix for the command name.
	Usage []string
	// If true then the given usages will replace the auto generated usage. Otherwise the given usages will be appended to the auto generated usage.
	ReplaceAutoGeneratedUsage bool
}

type PluginSignature struct {
	Name  string `json:"name,omitempty"`
	Usage string `json:"usage,omitempty"`
	// Only used internally in the CLI.
	ExecutablePath string `json:"executablePath,omitempty"`
}
