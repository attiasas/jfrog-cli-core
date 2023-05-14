package mvn

import (
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/generic"
	commandsutils "github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/utils"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/ioutils"
	mvnutils "github.com/jfrog/jfrog-cli-core/v2/utils/mvn"
	xrutils "github.com/jfrog/jfrog-cli-core/v2/xray/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/spf13/viper"
)

type MvnCommand struct {
	goals              []string
	configPath         string
	insecureTls        bool
	configuration      *utils.BuildConfiguration
	serverDetails      *config.ServerDetails
	threads            int
	detailedSummary    bool
	xrayScan           bool
	scanOutputFormat   xrutils.OutputFormat
	result             *commandsutils.Result
	deploymentDisabled bool
	// File path for Maven extractor in which all build's artifacts details will be listed at the end of the build.
	buildArtifactsDetailsFile string
}

func NewMvnCommand() *MvnCommand {
	return &MvnCommand{}
}

func (mc *MvnCommand) SetServerDetails(serverDetails *config.ServerDetails) *MvnCommand {
	mc.serverDetails = serverDetails
	return mc
}

func (mc *MvnCommand) SetConfiguration(configuration *utils.BuildConfiguration) *MvnCommand {
	mc.configuration = configuration
	return mc
}

func (mc *MvnCommand) SetConfigPath(configPath string) *MvnCommand {
	mc.configPath = configPath
	return mc
}

func (mc *MvnCommand) SetGoals(goals []string) *MvnCommand {
	mc.goals = goals
	return mc
}

func (mc *MvnCommand) SetThreads(threads int) *MvnCommand {
	mc.threads = threads
	return mc
}

func (mc *MvnCommand) SetInsecureTls(insecureTls bool) *MvnCommand {
	mc.insecureTls = insecureTls
	return mc
}

func (mc *MvnCommand) SetDetailedSummary(detailedSummary bool) *MvnCommand {
	mc.detailedSummary = detailedSummary
	return mc
}

func (mc *MvnCommand) IsDetailedSummary() bool {
	return mc.detailedSummary
}

func (mc *MvnCommand) SetXrayScan(xrayScan bool) *MvnCommand {
	mc.xrayScan = xrayScan
	return mc
}

func (mc *MvnCommand) IsXrayScan() bool {
	return mc.xrayScan
}

func (mc *MvnCommand) SetScanOutputFormat(format xrutils.OutputFormat) *MvnCommand {
	mc.scanOutputFormat = format
	return mc
}

func (mc *MvnCommand) Result() *commandsutils.Result {
	return mc.result
}

func (mc *MvnCommand) setResult(result *commandsutils.Result) *MvnCommand {
	mc.result = result
	return mc
}

func (mc *MvnCommand) init() (vConfig *viper.Viper, err error) {
	// Read config
	vConfig, err = utils.ReadMavenConfig(mc.configPath, nil)
	if err != nil {
		return
	}
	if mc.IsXrayScan() && !vConfig.IsSet("deployer") {
		err = errorutils.CheckErrorf("Conditional upload can only be performed if deployer is set in the config")
		return
	}
	// Maven's extractor deploys build artifacts. This should be disabled since there is no intent to deploy anything or deploy upon Xray scan results.
	mc.deploymentDisabled = mc.IsXrayScan() || !vConfig.IsSet("deployer")
	if mc.shouldCreateBuildArtifactsFile() {
		// Created a file that will contain all the details about the build's artifacts
		tempFile, err := fileutils.CreateTempFile()
		if err != nil {
			return nil, err
		}
		// If this is a Windows machine there is a need to modify the path for the build info file to match Java syntax with double \\
		mc.buildArtifactsDetailsFile = ioutils.DoubleWinPathSeparator(tempFile.Name())
		if err = tempFile.Close(); errorutils.CheckError(err) != nil {
			return nil, err
		}
	}
	return
}

// Maven extractor generates the details of the build's artifacts.
// This is required for Xray scan and for the detailed summary.
// We can either scan or print the generated artifacts.
func (mc *MvnCommand) shouldCreateBuildArtifactsFile() bool {
	return (mc.IsDetailedSummary() && !mc.deploymentDisabled) || mc.IsXrayScan()
}

func (mc *MvnCommand) Run() error {
	vConfig, err := mc.init()
	if err != nil {
		return err
	}

	err = mvnutils.RunMvn(vConfig, mc.buildArtifactsDetailsFile, mc.configuration, mc.goals, mc.threads, mc.insecureTls, mc.deploymentDisabled)
	if err != nil {
		return err
	}
	if mc.buildArtifactsDetailsFile != "" {
		err = mc.unmarshalDeployableArtifacts(mc.buildArtifactsDetailsFile)
		if err != nil {
			return err
		}
		if mc.IsXrayScan() {
			return mc.conditionalUpload()
		}
	}
	return nil
}

// Returns the ServerDetails. The information returns from the config file provided.
func (mc *MvnCommand) ServerDetails() (*config.ServerDetails, error) {
	// Get the serverDetails from the config file.
	if mc.serverDetails == nil {
		vConfig, err := utils.ReadConfigFile(mc.configPath, utils.YAML)
		if err != nil {
			return nil, err
		}
		mc.serverDetails, err = utils.GetServerDetails(vConfig)
		if err != nil {
			return nil, err
		}
	}
	return mc.serverDetails, nil
}

func (mc *MvnCommand) unmarshalDeployableArtifacts(filesPath string) error {
	result, err := commandsutils.UnmarshalDeployableArtifacts(filesPath, mc.configPath, mc.IsXrayScan())
	if err != nil {
		return err
	}
	mc.setResult(result)
	return nil
}

func (mc *MvnCommand) CommandName() string {
	return "rt_maven"
}

// ConditionalUpload will scan the artifact using Xray and will upload them only if the scan passes with no
// violation.
func (mc *MvnCommand) conditionalUpload() error {
	// Initialize the server details (from config) if it hasn't been initialized yet.
	_, err := mc.ServerDetails()
	if err != nil {
		return err
	}
	binariesSpecFile, pomSpecFile, err := commandsutils.ScanDeployableArtifacts(mc.result, mc.serverDetails, mc.threads, mc.scanOutputFormat)
	// If the detailed summary wasn't requested, the reader should be closed here.
	// (otherwise it will be closed by the detailed summary print method)
	if !mc.IsDetailedSummary() {
		e := mc.result.Reader().Close()
		if e != nil {
			return e
		}
	} else {
		mc.result.Reader().Reset()
	}
	if err != nil {
		return err
	}
	// The case scan failed
	if binariesSpecFile == nil {
		return nil
	}
	// First upload binaries
	if len(binariesSpecFile.Files) > 0 {
		uploadCmd := generic.NewUploadCommand()
		uploadConfiguration := new(utils.UploadConfiguration)
		uploadConfiguration.Threads = mc.threads
		uploadCmd.SetUploadConfiguration(uploadConfiguration).SetBuildConfiguration(mc.configuration).SetSpec(binariesSpecFile).SetServerDetails(mc.serverDetails)
		err = uploadCmd.Run()
		if err != nil {
			return err
		}
	}
	if len(pomSpecFile.Files) > 0 {
		// Then Upload pom.xml's
		uploadCmd := generic.NewUploadCommand()
		uploadConfiguration := new(utils.UploadConfiguration)
		uploadConfiguration.Threads = mc.threads
		uploadCmd.SetUploadConfiguration(uploadConfiguration).SetBuildConfiguration(mc.configuration).SetSpec(pomSpecFile).SetServerDetails(mc.serverDetails)
		err = uploadCmd.Run()
	}
	return err
}
