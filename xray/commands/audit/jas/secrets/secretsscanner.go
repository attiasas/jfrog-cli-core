package secrets

import (
	"path/filepath"

	"github.com/jfrog/jfrog-cli-core/v2/xray/commands/audit/jas"
	"github.com/jfrog/jfrog-cli-core/v2/xray/utils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/owenrumney/go-sarif/v2/sarif"
)

const (
	secretsScanCommand = "sec"
	secretsScannerType = "secrets-scan"
)

type SecretScanManager struct {
	secretsScannerResults []*sarif.Run
	scanner               *jas.JasScanner
}

// The getSecretsScanResults function runs the secrets scan flow, which includes the following steps:
// Creating an SecretScanManager object.
// Running the analyzer manager executable.
// Parsing the analyzer manager results.
// Return values:
// []utils.IacOrSecretResult: a list of the secrets that were found.
// error: An error object (if any).
func RunSecretsScan(scanner *jas.JasScanner) (results []*sarif.Run, err error) {
	secretScanManager := newSecretsScanManager(scanner)
	log.Info("Running secrets scanning...")
	if err = secretScanManager.scanner.Run(secretScanManager); err != nil {
		err = utils.ParseAnalyzerManagerError(utils.Secrets, err)
		return
	}
	results = secretScanManager.secretsScannerResults
	if len(results) > 0 {
		log.Info("Found", len(results), "secrets")
	}
	return
}

func newSecretsScanManager(scanner *jas.JasScanner) (manager *SecretScanManager) {
	return &SecretScanManager{
		secretsScannerResults: []*sarif.Run{},
		scanner:               scanner,
	}
}

func (s *SecretScanManager) Run(wd string) (err error) {
	scanner := s.scanner
	if err = s.createConfigFile(wd); err != nil {
		return
	}
	if err = s.runAnalyzerManager(); err != nil {
		return
	}
	workingDirRuns, err := utils.ReadScanRunsFromFile(scanner.ResultsFileName)
	if err != nil {
		return
	}
	processSecretScanRuns(workingDirRuns, wd)
	s.secretsScannerResults = append(s.secretsScannerResults, workingDirRuns...)
	return
}

type secretsScanConfig struct {
	Scans []secretsScanConfiguration `yaml:"scans"`
}

type secretsScanConfiguration struct {
	Roots       []string `yaml:"roots"`
	Output      string   `yaml:"output"`
	Type        string   `yaml:"type"`
	SkippedDirs []string `yaml:"skipped-folders"`
}

func (s *SecretScanManager) createConfigFile(currentWd string) error {
	configFileContent := secretsScanConfig{
		Scans: []secretsScanConfiguration{
			{
				Roots:       []string{currentWd},
				Output:      s.scanner.ResultsFileName,
				Type:        secretsScannerType,
				SkippedDirs: jas.SkippedDirs,
			},
		},
	}
	return jas.CreateScannersConfigFile(s.scanner.ConfigFileName, configFileContent)
}

func (s *SecretScanManager) runAnalyzerManager() error {
	return s.scanner.AnalyzerManager.Exec(s.scanner.ConfigFileName, secretsScanCommand, filepath.Dir(s.scanner.AnalyzerManager.AnalyzerManagerFullPath), s.scanner.ServerDetails)
}

func processSecretScanRuns(sarifRuns []*sarif.Run, wd string) {
	for _, secretRun := range sarifRuns {
		// Change general attributes
		jas.ProcessJasScanRun(secretRun, wd)
		// Change specific scan attributes
		for _, secretResult := range secretRun.Results {
			for _, location := range secretResult.Locations {
				secret := utils.GetLocationSnippetPointer(location)
				utils.SetLocationSnippet(location, jas.HideSecret(*secret))
			}
		}
	}
}