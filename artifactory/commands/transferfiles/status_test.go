package transferfiles

import (
	"bytes"
	"testing"

	"github.com/jfrog/build-info-go/utils"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/transferfiles/state"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/tests"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/stretchr/testify/assert"
)

func initStatusTest(t *testing.T) (*bytes.Buffer, func()) {
	cleanUpJfrogHome, err := tests.SetJfrogHome()
	assert.NoError(t, err)

	// Create transfer directory
	transferDir, err := coreutils.GetJfrogTransferDir()
	assert.NoError(t, err)
	err = utils.CreateDirIfNotExist(transferDir)
	assert.NoError(t, err)

	// Redirect log to buffer
	buffer, _, previousLog := tests.RedirectLogOutputToBuffer()
	return buffer, func() {
		log.SetLogger(previousLog)
		cleanUpJfrogHome()
	}
}

func TestShowStatusNotRunning(t *testing.T) {
	buffer, cleanUp := initStatusTest(t)
	defer cleanUp()

	// Run show status and check output
	assert.NoError(t, ShowStatus())
	assert.Contains(t, buffer.String(), "Status:Not running")
}

func TestShowStatus(t *testing.T) {
	buffer, cleanUp := initStatusTest(t)
	defer cleanUp()

	// Create state manager and persist to file system
	createStateManager(t, FullTransferPhase)

	// Run show status and check output
	assert.NoError(t, ShowStatus())
	results := buffer.String()

	// Check overall status
	assert.Contains(t, results, "Overall Transfer Status")
	assert.Contains(t, results, "Status:		Running")
	assert.Contains(t, results, "Start time:		")
	assert.Contains(t, results, "Storage:		4.9 KiB / 10.9 KiB (45.0%)")
	assert.Contains(t, results, "Repositories:	15 / 1111 (1.4%)")
	assert.Contains(t, results, "Working threads:	16")
	assert.Contains(t, results, "Transfer failures:	223")

	// Check repository status
	assert.Contains(t, results, "Current Repository Status")
	assert.Contains(t, results, "Name:		repo1")
	assert.Contains(t, results, "Phase:		Transferring all files in the repository (1/3)")
	assert.Contains(t, results, "Storage:		4.9 KiB / 9.8 KiB (50.0%)")
	assert.Contains(t, results, "Files:		500 / 10000 (5.0%)")
}

func TestShowStatusDiffPhase(t *testing.T) {
	buffer, cleanUp := initStatusTest(t)
	defer cleanUp()

	// Create state manager and persist to file system
	createStateManager(t, FilesDiffPhase)

	// Run show status and check output
	assert.NoError(t, ShowStatus())
	results := buffer.String()

	// Check overall status
	assert.Contains(t, results, "Overall Transfer Status")
	assert.Contains(t, results, "Status:		Running")
	assert.Contains(t, results, "Start time:		")
	assert.Contains(t, results, "Storage:		4.9 KiB / 10.9 KiB (45.0%)")
	assert.Contains(t, results, "Repositories:	15 / 1111 (1.4%)")
	assert.Contains(t, results, "Working threads:	16")
	assert.Contains(t, results, "Transfer failures:	223")

	// Check repository status
	assert.Contains(t, results, "Current Repository Status")
	assert.Contains(t, results, "Name:		repo1")
	assert.Contains(t, results, "Phase:		Transferring newly created and modified files (2/3)")
	assert.NotContains(t, results, "Storage:		4.9 KiB / 9.8 KiB (50.0%)")
	assert.NotContains(t, results, "Files:		500 / 10000 (5.0%)")
}

// Create state manager and persist in the file system.
// t     - The testing object
// phase - Phase ID
func createStateManager(t *testing.T, phase int) {
	stateManager, err := state.NewTransferStateManager(false)
	assert.NoError(t, err)
	assert.NoError(t, stateManager.TryLockTransferStateManager())
	assert.NoError(t, stateManager.SetRepoState(repo1Key, 10000, 10000, false))

	stateManager.CurrentRepo = repo1Key
	stateManager.CurrentRepoPhase = phase
	stateManager.TotalSizeBytes = 11111
	stateManager.TotalUnits = 1111
	stateManager.TransferredUnits = 15
	stateManager.WorkingThreads = 16
	stateManager.TransferFailures = 223

	// Increment transferred suze and files. This action also persists the run status.
	assert.NoError(t, stateManager.IncTransferredSizeAndFiles(repo1Key, 500, 5000))

	// Save transfer state.
	assert.NoError(t, stateManager.SaveState())
}

func TestSizeToString(t *testing.T) {
	testCases := []struct {
		sizeInBytes int64
		expected    string
	}{
		{0, "0.0 KiB"},
		{10, "0.0 KiB"},
		{100, "0.1 KiB"},
		{1000, "1.0 KiB"},
		{1024, "1.0 KiB"},
		{1025, "1.0 KiB"},
		{4000, "3.9 KiB"},
		{4096, "4.0 KiB"},
		{1000000, "976.6 KiB"},
		{1048576, "1.0 MiB"},
		{1073741824, "1.0 GiB"},
		{1073741824, "1.0 GiB"},
		{1099511627776, "1.0 TiB"},
		{1125899906842624, "1.0 PiB"},
		{1125899906842624, "1.0 PiB"},
		{1.152921504606847e18, "1.0 EiB"},
	}
	for _, testCase := range testCases {
		assert.Equal(t, sizeToString(testCase.sizeInBytes), testCase.expected)
	}
}
