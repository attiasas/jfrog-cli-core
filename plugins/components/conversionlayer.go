package components

import (
	"errors"
	"fmt"
	"strings"

	"github.com/jfrog/gofrog/datastructures"
	"github.com/jfrog/jfrog-cli-core/v2/docs/common"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/urfave/cli"
)

func ConvertApp(jfrogApp App) (*cli.App, error) {
	var err error
	app := cli.NewApp()
	app.Name = jfrogApp.Name
	app.Description = jfrogApp.Description
	app.Version = jfrogApp.Version
	app.Commands, err = ConvertAppCommands(jfrogApp)
	if err != nil {
		return nil, err
	}
	// Defaults:
	app.EnableBashCompletion = true
	return app, nil
}

func ConvertAppCommands(jfrogApp App, commandPrefix ...string) (cmds []cli.Command, err error) {
	cmds, err = convertCommands(jfrogApp.Commands, commandPrefix...)
	if err != nil || len(jfrogApp.Subcommands) == 0 {
		return
	}
	subcommands, err := convertSubcommands(jfrogApp.Subcommands, commandPrefix...)
	if err != nil {
		return
	}
	cmds = append(cmds, subcommands...)
	return
}

func convertSubcommands(subcommands []Namespace, nameSpaces ...string) ([]cli.Command, error) {
	var converted []cli.Command
	for _, ns := range subcommands {
		nameSpaceCommand := cli.Command{
			Name:     ns.Name,
			Usage:    ns.Description,
			Hidden:   ns.Hidden,
			Category: ns.Category,
		}
		nsCommands, err := convertCommands(ns.Commands, append(nameSpaces, ns.Name)...)
		if err != nil {
			return converted, err
		}
		nameSpaceCommand.Subcommands = nsCommands
		converted = append(converted, nameSpaceCommand)
	}
	return converted, nil
}

func convertCommands(commands []Command, nameSpaces ...string) ([]cli.Command, error) {
	var converted []cli.Command
	for _, cmd := range commands {
		cur, err := convertCommand(cmd, nameSpaces...)
		if err != nil {
			return converted, err
		}
		converted = append(converted, cur)
	}
	return converted, nil
}

func convertCommand(cmd Command, namespaces ...string) (cli.Command, error) {
	convertedFlags, convertedStringFlags, err := convertFlags(cmd)
	if err != nil {
		return cli.Command{}, err
	}
	cmdUsages, err := createCommandUsages(cmd, convertedStringFlags, namespaces...)
	if err != nil {
		return cli.Command{}, err
	}
	return cli.Command{
		Name:            cmd.Name,
		Flags:           convertedFlags,
		Aliases:         cmd.Aliases,
		Category:        cmd.Category,
		Usage:           cmd.Description,
		Description:     cmd.Description,
		HelpName:        common.CreateUsage(getCmdUsageString(cmd, namespaces...), cmd.Description, cmdUsages),
		UsageText:       createArgumentsSummary(cmd),
		ArgsUsage:       createEnvVarsSummary(cmd),
		BashComplete:    common.CreateBashCompletionFunc(),
		SkipFlagParsing: cmd.SkipFlagParsing,
		Hidden:          cmd.Hidden,
		// Passing any other interface than 'cli.ActionFunc' will fail the command.
		Action: getActionFunc(cmd),
	}, nil
}

func removeEmptyValues(slice []string) []string {
	var result []string
	for _, s := range slice {
		if s != "" {
			result = append(result, s)
		}
	}
	return result
}

// Create the command usage strings that will be shown in the help.
func createCommandUsages(cmd Command, convertedStringFlags map[string]StringFlag, namespaces ...string) (usages []string, err error) {
	// Handle manual usages provided.
	if cmd.UsageOptions != nil {
		for _, manualUsage := range cmd.UsageOptions.Usage {
			usages = append(usages, fmt.Sprintf("%s %s", coreutils.GetCliExecutableName(), manualUsage))
		}
		if cmd.UsageOptions.ReplaceAutoGeneratedUsage {
			return
		}
	}
	// Handle auto generated usages for the command.
	generated, err := generateCommandUsages(getCmdUsageString(cmd, namespaces...), cmd, convertedStringFlags)
	if err != nil {
		return
	}
	usages = append(usages, generated...)
	return
}

func getCmdUsageString(cmd Command, namespaces ...string) string {
	return strings.Join(append(removeEmptyValues(namespaces), cmd.Name), " ")
}

// Generated usages are based on the command's flags and arguments:
// <cli-name> <command-name> [command options] --mandatory-opt1=<opt1-value-alias> --mandatory-opt2=<value>... <arg1> [optional-arg2] <arg3>...
func generateCommandUsages(usagePrefix string, cmd Command, convertedStringFlags map[string]StringFlag) (usages []string, err error) {
	argumentsUsageParts, flagReplacements, err := getArgumentsUsageParts(cmd, convertedStringFlags)
	if err != nil {
		return
	}
	if len(argumentsUsageParts) == 0 {
		// No arguments provided.
		usages = append(usages, fmt.Sprintf("%s%s", usagePrefix, getFlagUsagePart(cmd, convertedStringFlags, nil)))
		return
	}
	usages = append(usages, fmt.Sprintf("%s%s%s", usagePrefix, getFlagUsagePart(cmd, convertedStringFlags, nil), argumentsUsageParts[0]))
	if len(argumentsUsageParts) == 1 {
		// No flag replacements, return single usage.
		return
	}
	// Add the usage with the flag replacements.
	usages = append(usages, fmt.Sprintf("%s%s%s", usagePrefix, getFlagUsagePart(cmd, convertedStringFlags, flagReplacements), argumentsUsageParts[1]))
	return
}

// Get the command usage parts that are related to arguments, if any.
// Mandatory arguments represented as <Arg-Name> and optional arguments represented as [Arg-Name].
// If some arguments have flag replacements, creates two parts: with and without all replacements:
// 1) <arg1> [optional-arg2] <arg3>
// 2) --<optional-flag-replacement-2-3>=<value> <arg1>
func getArgumentsUsageParts(cmd Command, convertedStringFlags map[string]StringFlag) (usageParts []string, flagReplacements *datastructures.Set[string], err error) {
	var usage string
	if usage = getArgsUsagePart(cmd); usage != "" {
		// No replacements arguments usage part. (1)
		usageParts = append(usageParts, usage)
	}
	if usage, flagReplacements, err = getArgsUsagePartWithReplacements(cmd, convertedStringFlags); err != nil {
		return
	}
	if usage != "" {
		// With replacements arguments usage part. (2)
		usageParts = append(usageParts, usage)
	}
	return
}

func getArgsUsagePart(cmd Command) (usage string) {
	for _, argument := range cmd.Arguments {
		usage += getArgumentUsage(argument)
	}
	return
}

func getArgumentUsage(argument Argument) string {
	if argument.Optional {
		return fmt.Sprintf(" [%s]", argument.Name)
	}
	return fmt.Sprintf(" <%s>", argument.Name)
}

func getArgsUsagePartWithReplacements(cmd Command, convertedStringFlags map[string]StringFlag) (usage string, flagReplacements *datastructures.Set[string], err error) {
	flagReplacements = datastructures.MakeSet[string]()
	for _, argument := range cmd.Arguments {
		if argument.ReplaceWithFlag == "" {
			usage += getArgumentUsage(argument)
			continue
		}
		if flagReplacements.Exists(argument.ReplaceWithFlag) {
			// Flag already exists in the replacements, skip. (Multiple arguments can have the same replacement flag)
			continue
		}
		if _, exists := convertedStringFlags[argument.ReplaceWithFlag]; !exists {
			err = fmt.Errorf("command '%s': argument '%s' has a defined replacement flag '%s' that does not exist", cmd.Name, argument.Name, argument.ReplaceWithFlag)
			return
		}
		flagReplacements.Add(argument.ReplaceWithFlag)
	}
	if flagReplacements.Size() == 0 {
		// No replacements, return empty string.
		return "", nil, nil
	}
	for _, flagName := range flagReplacements.ToSlice() {
		usage = getMandatoryFlagUsage(convertedStringFlags[flagName]) + usage
	}
	return
}

// Get the command usage part that is related to flags, if any.
// If some flags are optional, returns with general prefix `[command options]` followed by the mandatory flags.
// Mandatory flags are returned with their value alias, if provided. --<Name>=<ValueAlias> or --<Name>=<value> if no alias.
func getFlagUsagePart(cmd Command, convertedStringFlags map[string]StringFlag, flagReplacements *datastructures.Set[string]) (usage string) {
	// Calculate flag counts.
	totalFlagCount := len(cmd.Flags)
	if totalFlagCount == 0 {
		return
	}
	mandatoryFlagCount := getMandatoryFlagCount(cmd)
	optionalFlagCount := totalFlagCount - mandatoryFlagCount
	optionalFlagCountUsedAsArgReplacements := 0
	if flagReplacements != nil {
		optionalFlagCountUsedAsArgReplacements = flagReplacements.Size()
	}
	// Add general prefix.
	if optionalFlagCount-optionalFlagCountUsedAsArgReplacements > 0 {
		usage += " [command options]"
	}
	if mandatoryFlagCount == 0 {
		return
	}
	// Add mandatory flags.
	for flagName, flag := range convertedStringFlags {
		if flag.Mandatory {
			valueAlias := "value"
			if flag.HelpValue != "" {
				valueAlias = flag.HelpValue
			}
			usage += fmt.Sprintf(" --%s=<%s>", flagName, valueAlias)
		}
	}
	return
}

func getMandatoryFlagCount(cmd Command) int {
	count := 0
	for _, flag := range cmd.Flags {
		if flag.IsMandatory() {
			count++
		}
	}
	return count
}

func getMandatoryFlagUsage(flag StringFlag) string {
	valueAlias := "value"
	if flag.HelpValue != "" {
		valueAlias = flag.HelpValue
	}
	return fmt.Sprintf(" --%s=<%s>", flag.Name, valueAlias)
}

func createArgumentsSummary(cmd Command) string {
	summary := ""
	for i, argument := range cmd.Arguments {
		if i > 0 {
			summary += "\n"
		}
		optional := ""
		if argument.Optional {
			optional = " [Optional]"
		}
		summary += "\t" + argument.Name + optional + "\n\t\t" + argument.Description + "\n"
	}
	return summary
}

func createEnvVarsSummary(cmd Command) string {
	var envVarsSummary []string
	for i, env := range cmd.EnvVars {
		summary := ""
		if i > 0 {
			summary += "\n"
		}
		summary += "\t" + env.Name + "\n"
		if env.Default != "" {
			summary += "\t\t[Default: " + env.Default + "]\n"
		}
		summary += "\t\t" + env.Description
		envVarsSummary = append(envVarsSummary, summary)
	}
	return strings.Join(envVarsSummary, "\n")
}

func convertFlags(cmd Command) ([]cli.Flag, map[string]StringFlag, error) {
	var convertedFlags []cli.Flag
	convertedStringFlags := map[string]StringFlag{}
	for _, flag := range cmd.Flags {
		converted, convertedString, err := convertByType(flag)
		if err != nil {
			return convertedFlags, convertedStringFlags, fmt.Errorf("command '%s': %w", cmd.Name, err)
		}
		if converted != nil {
			convertedFlags = append(convertedFlags, converted)
		}
		if convertedString != nil {
			convertedStringFlags[flag.GetName()] = *convertedString
		}
	}
	return convertedFlags, convertedStringFlags, nil
}

func convertByType(flag Flag) (cli.Flag, *StringFlag, error) {
	switch actualType := flag.(type) {
	case StringFlag:
		return convertStringFlag(actualType), &actualType, nil
	case BoolFlag:
		return convertBoolFlag(actualType), nil, nil
	}
	return nil, nil, errorutils.CheckErrorf("flag '%s' does not match any known flag type", flag.GetName())
}

func convertStringFlag(f StringFlag) cli.Flag {
	stringFlag := cli.StringFlag{
		Name:   f.Name,
		Hidden: f.Hidden,
		Usage:  f.Description + "` `",
	}
	// If default is set, add its value and return.
	if f.DefaultValue != "" {
		stringFlag.Usage = fmt.Sprintf("[Default: %s] %s", f.DefaultValue, stringFlag.Usage)
		return stringFlag
	}
	// Otherwise, mark as mandatory/optional accordingly.
	if f.Mandatory {
		stringFlag.Usage = "[Mandatory] " + stringFlag.Usage
	} else {
		stringFlag.Usage = "[Optional] " + stringFlag.Usage
	}
	return stringFlag
}

func convertBoolFlag(f BoolFlag) cli.Flag {
	if f.DefaultValue {
		return cli.BoolTFlag{
			Name:   f.Name,
			Hidden: f.Hidden,
			Usage:  "[Default: true] " + f.Description + "` `",
		}
	}
	return cli.BoolFlag{
		Name:   f.Name,
		Hidden: f.Hidden,
		Usage:  "[Default: false] " + f.Description + "` `",
	}
}

// Wrap the base's ActionFunc with our own, while retrieving needed information from the Context.
func getActionFunc(cmd Command) cli.ActionFunc {
	return func(baseContext *cli.Context) error {
		pluginContext, err := ConvertContext(baseContext, cmd.Flags...)
		if err != nil {
			return err
		}
		return cmd.Action(pluginContext)
	}
}

func ConvertContext(baseContext *cli.Context, flagsToConvert ...Flag) (*Context, error) {
	pluginContext := &Context{
		CommandName:      baseContext.Command.Name,
		Arguments:        baseContext.Args(),
		PrintCommandHelp: getPrintCommandHelpFunc(baseContext),
	}
	return pluginContext, fillFlagMaps(pluginContext, baseContext, flagsToConvert)
}

func getPrintCommandHelpFunc(c *cli.Context) func(commandName string) error {
	return func(commandName string) error {
		return cli.ShowCommandHelp(c, c.Command.Name)
	}
}

func fillFlagMaps(c *Context, baseContext *cli.Context, originalFlags []Flag) error {
	c.stringFlags = make(map[string]string)
	c.boolFlags = make(map[string]bool)

	// Loop over all plugin's known flags.
	for _, flag := range originalFlags {
		if stringFlag, ok := flag.(StringFlag); ok {
			finalValue, skip, err := getValueForStringFlag(stringFlag, baseContext)
			if err != nil {
				return err
			}
			if skip {
				continue
			}
			c.stringFlags[stringFlag.Name] = finalValue
		}

		if boolFlag, ok := flag.(BoolFlag); ok {
			finalValue, skip := getValueForBoolFlag(boolFlag, baseContext)
			if !skip {
				c.boolFlags[boolFlag.Name] = finalValue
			}
		}
	}
	return nil
}

func getValueForStringFlag(f StringFlag, baseContext *cli.Context) (finalValue string, skip bool, err error) {
	value := baseContext.String(f.Name)
	if value != "" {
		finalValue = value
		return
	}
	if f.DefaultValue != "" {
		// Empty but has a default value defined.
		finalValue = f.DefaultValue
		return
	}
	skip = !baseContext.IsSet(f.Name)
	// Empty but mandatory.
	if f.Mandatory {
		err = errors.New("Mandatory flag '" + f.Name + "' is missing")
	}
	return
}

func getValueForBoolFlag(f BoolFlag, baseContext *cli.Context) (finalValue, skip bool) {
	if f.DefaultValue {
		finalValue = baseContext.BoolT(f.Name)
		return
	}
	skip = !baseContext.IsSet(f.Name)
	finalValue = baseContext.Bool(f.Name)
	return
}
