package utils

import (
	"testing"

	"github.com/owenrumney/go-sarif/v2/sarif"
	"github.com/stretchr/testify/assert"
)

func TestAggregateMultipleRunsIntoSingle(t *testing.T) {
	tests := []struct {
		runs           []*sarif.Run
		expectedOutput *sarif.Run
	}{
		{
			runs:           []*sarif.Run{},
			expectedOutput: CreateRunWithDummyResults(),
		},
		{
			runs: []*sarif.Run{
				CreateRunWithDummyResults(
					CreateDummyPassingResult("rule1"),
					CreateDummyResultWithOneLocation("file", 1, 2, 3, 4, "snippet", "rule2", "level"),
				).WithInvocations([]*sarif.Invocation{
					sarif.NewInvocation().WithWorkingDirectory(sarif.NewSimpleArtifactLocation("wd")),
				}),
				CreateRunWithDummyResults(),
			},
			expectedOutput: CreateRunWithDummyResults(
				CreateDummyPassingResult("rule1"),
				CreateDummyResultWithOneLocation("file", 1, 2, 3, 4, "snippet", "rule2", "level"),
			).WithInvocations([]*sarif.Invocation{
				sarif.NewInvocation().WithWorkingDirectory(sarif.NewSimpleArtifactLocation("wd")),
			}),
		},
		{
			runs: []*sarif.Run{
				CreateRunWithDummyResults(
					CreateDummyPassingResult("rule1"),
					CreateDummyResultWithOneLocation("file", 1, 2, 3, 4, "snippet", "rule2", "level"),
					CreateDummyResultWithOneLocation("file", 1, 2, 3, 4, "snippet", "rule3", "level"),
				).WithInvocations([]*sarif.Invocation{
					sarif.NewInvocation().WithWorkingDirectory(sarif.NewSimpleArtifactLocation("wd")),
				}),
				CreateRunWithDummyResults(
					CreateDummyResultWithLocations("msg", "rule2", "level",
						CreateDummyLocation("file", 1, 2, 3, 4, "snippet"),
						CreateDummyLocation("file2", 1, 2, 3, 4, "other-snippet"),
					),
					CreateDummyResultWithOneLocation("file", 5, 6, 7, 8, "snippet2", "rule2", "level"),
				).WithInvocations([]*sarif.Invocation{
					sarif.NewInvocation().WithWorkingDirectory(sarif.NewSimpleArtifactLocation("wd2")),
				}),
			},
			expectedOutput: CreateRunWithDummyResults(
				// First run results
				CreateDummyPassingResult("rule1"),
				CreateDummyResultWithOneLocation("file", 1, 2, 3, 4, "snippet", "rule2", "level"),
				CreateDummyResultWithOneLocation("file", 1, 2, 3, 4, "snippet", "rule3", "level"),
				// Second run results
				CreateDummyResultWithLocations("msg", "rule2", "level",
					CreateDummyLocation("file", 1, 2, 3, 4, "snippet"),
					CreateDummyLocation("file2", 1, 2, 3, 4, "other-snippet"),
				),
				CreateDummyResultWithOneLocation("file", 5, 6, 7, 8, "snippet2", "rule2", "level"),
			).WithInvocations([]*sarif.Invocation{
				sarif.NewInvocation().WithWorkingDirectory(sarif.NewSimpleArtifactLocation("wd")),
				sarif.NewInvocation().WithWorkingDirectory(sarif.NewSimpleArtifactLocation("wd2")),
			}),
		},
	}

	for _, test := range tests {
		run := CreateRunWithDummyResults()
		AggregateMultipleRunsIntoSingle(test.runs, run)
		assert.Equal(t, test.expectedOutput, run)
	}
}

func TestGetLocationRelatedCodeFlowsFromResult(t *testing.T) {
	tests := []struct {
		result         *sarif.Result
		location       *sarif.Location
		expectedOutput []*sarif.CodeFlow
	}{
		{
			result:         CreateDummyPassingResult("rule"),
			location:       CreateDummyLocation("file", 0, 0, 0, 0, "snippet"),
			expectedOutput: nil,
		},
		{
			result:         CreateDummyResultWithOneLocation("file", 0, 0, 0, 0, "snippet", "rule", "level"),
			location:       CreateDummyLocation("file", 0, 0, 0, 0, "snippet"),
			expectedOutput: nil,
		},
		{
			result:         CreateDummyResultWithOneLocation("file", 0, 0, 0, 0, "snippet", "rule", "level").WithCodeFlows([]*sarif.CodeFlow{CreateDummyCodeFlow(CreateDummyThreadFlow(CreateDummyLocation("file", 0, 0, 0, 0, "snippet")))}),
			location:       CreateDummyLocation("file2", 0, 0, 0, 0, "snippet"),
			expectedOutput: nil,
		},
		{
			result:         CreateDummyResultWithOneLocation("file", 0, 0, 0, 0, "snippet", "rule", "level").WithCodeFlows([]*sarif.CodeFlow{CreateDummyCodeFlow(CreateDummyThreadFlow(CreateDummyLocation("file", 0, 0, 0, 0, "snippet")))}),
			location:       CreateDummyLocation("file", 0, 0, 0, 0, "snippet"),
			expectedOutput: []*sarif.CodeFlow{CreateDummyCodeFlow(CreateDummyThreadFlow(CreateDummyLocation("file", 0, 0, 0, 0, "snippet")))},
		},
		{
			result: CreateDummyResultWithOneLocation("file", 0, 0, 0, 0, "snippet", "rule", "level").WithCodeFlows([]*sarif.CodeFlow{
				CreateDummyCodeFlow(CreateDummyThreadFlow(
					CreateDummyLocation("file4", 2, 0, 2, 0, "snippetB"),
					CreateDummyLocation("file2", 0, 2, 0, 2, "snippetA"),
					CreateDummyLocation("file", 0, 0, 0, 0, "snippet"),
				)),
				CreateDummyCodeFlow(CreateDummyThreadFlow(
					CreateDummyLocation("file", 0, 0, 0, 0, "snippet"),
					CreateDummyLocation("file2", 1, 0, 1, 0, "snippet"),
				)),
				CreateDummyCodeFlow(CreateDummyThreadFlow(
					CreateDummyLocation("fileC", 1, 1, 1, 1, "snippetC"),
					CreateDummyLocation("file", 0, 0, 0, 0, "snippet"),
				)),
			}),
			location: CreateDummyLocation("file", 0, 0, 0, 0, "snippet"),
			expectedOutput: []*sarif.CodeFlow{
				CreateDummyCodeFlow(CreateDummyThreadFlow(
					CreateDummyLocation("file4", 2, 0, 2, 0, "snippetB"),
					CreateDummyLocation("file2", 0, 2, 0, 2, "snippetA"),
					CreateDummyLocation("file", 0, 0, 0, 0, "snippet"),
				)),
				CreateDummyCodeFlow(CreateDummyThreadFlow(
					CreateDummyLocation("fileC", 1, 1, 1, 1, "snippetC"),
					CreateDummyLocation("file", 0, 0, 0, 0, "snippet"),
				)),
			},
		},
	}

	for _, test := range tests {
		assert.Equal(t, test.expectedOutput, GetLocationRelatedCodeFlowsFromResult(test.location, test.result))
	}
}

func TestGetResultsLocationCount(t *testing.T) {
	tests := []struct {
		runs           []*sarif.Run
		expectedOutput int
	}{
		{
			runs:           []*sarif.Run{},
			expectedOutput: 0,
		},
		{
			runs:           []*sarif.Run{CreateRunWithDummyResults()},
			expectedOutput: 0,
		},
		{
			runs: []*sarif.Run{CreateRunWithDummyResults(
				CreateDummyPassingResult("rule"),
				CreateDummyResultWithOneLocation("file", 0, 0, 0, 0, "snippet", "rule", "level"),
			)},
			expectedOutput: 1,
		},
		{
			runs: []*sarif.Run{
				CreateRunWithDummyResults(
					CreateDummyPassingResult("rule"),
					CreateDummyResultWithOneLocation("file", 0, 0, 0, 0, "snippet", "rule", "level"),
				),
				CreateRunWithDummyResults(
					CreateDummyResultWithLocations(
						"msg",
						"rule",
						"level",
						CreateDummyLocation("file", 0, 0, 0, 0, "snippet"),
						CreateDummyLocation("file", 0, 0, 0, 0, "snippet"),
						CreateDummyLocation("file", 0, 0, 0, 0, "snippet"),
					),
				),
			},
			expectedOutput: 4,
		},
	}

	for _, test := range tests {
		assert.Equal(t, test.expectedOutput, GetResultsLocationCount(test.runs...))
	}
}

func TestGetResultMsgText(t *testing.T) {
	tests := []struct {
		result         *sarif.Result
		expectedOutput string
	}{
		{
			result:         &sarif.Result{},
			expectedOutput: "",
		},
		{
			result:         CreateDummyResultWithLocations("msg", "rule", "level"),
			expectedOutput: "msg",
		},
	}

	for _, test := range tests {
		assert.Equal(t, test.expectedOutput, GetResultMsgText(test.result))
	}
}

func TestGetLocationSnippet(t *testing.T) {
	tests := []struct {
		location       *sarif.Location
		expectedOutput string
	}{
		{
			location:       nil,
			expectedOutput: "",
		},
		{
			location:       CreateDummyLocation("filename", 1, 2, 3, 4, "snippet"),
			expectedOutput: "snippet",
		},
	}

	for _, test := range tests {
		assert.Equal(t, test.expectedOutput, GetLocationSnippet(test.location))
	}
}

func TestSetLocationSnippet(t *testing.T) {
	tests := []struct {
		location       *sarif.Location
		expectedOutput string
	}{
		{
			location:       nil,
			expectedOutput: "",
		},
		{
			location:       CreateDummyLocation("filename", 1, 2, 3, 4, "snippet"),
			expectedOutput: "changedSnippet",
		},
	}

	for _, test := range tests {
		SetLocationSnippet(test.location, test.expectedOutput)
		assert.Equal(t, test.expectedOutput, GetLocationSnippet(test.location))
	}
}

func TestGetLocationFileName(t *testing.T) {
	tests := []struct {
		location       *sarif.Location
		expectedOutput string
	}{
		{
			location:       nil,
			expectedOutput: "",
		},
		{
			location:       CreateDummyLocation("filename", 1, 2, 3, 4, "snippet"),
			expectedOutput: "filename",
		},
	}

	for _, test := range tests {
		assert.Equal(t, test.expectedOutput, GetLocationFileName(test.location))
	}
}

func TestGetRelativeLocationFileName(t *testing.T) {
	tests := []struct {
		location       *sarif.Location
		invocations    []*sarif.Invocation
		expectedOutput string
	}{
		{
			location:       CreateDummyLocation("file:///root/someDir/another/file", 1, 2, 3, 4, "snippet"),
			invocations:    []*sarif.Invocation{},
			expectedOutput: "root/someDir/another/file",
		},
		{
			location:       CreateDummyLocation("file:///root/someDir/another/file", 1, 2, 3, 4, "snippet"),
			invocations:    []*sarif.Invocation{{WorkingDirectory: sarif.NewSimpleArtifactLocation("/not/relevant")}},
			expectedOutput: "root/someDir/another/file",
		},
		{
			location:       CreateDummyLocation("file:///root/someDir/another/file", 1, 2, 3, 4, "snippet"),
			invocations:    []*sarif.Invocation{{WorkingDirectory: sarif.NewSimpleArtifactLocation("/root/someDir/")}},
			expectedOutput: "another/file",
		},
	}

	for _, test := range tests {
		assert.Equal(t, test.expectedOutput, GetRelativeLocationFileName(test.location, test.invocations))
	}
}

func TestSetLocationFileName(t *testing.T) {
	tests := []struct {
		location       *sarif.Location
		expectedOutput string
	}{
		{
			location:       nil,
			expectedOutput: "",
		},
		{
			location:       CreateDummyLocation("filename", 1, 2, 3, 4, "snippet"),
			expectedOutput: "changedFilename",
		},
	}

	for _, test := range tests {
		SetLocationFileName(test.location, test.expectedOutput)
		assert.Equal(t, test.expectedOutput, GetLocationFileName(test.location))
	}
}

func TestGetLocationRegion(t *testing.T) {
	tests := []struct {
		location       *sarif.Location
		expectedOutput *sarif.Region
	}{
		{
			location:       nil,
			expectedOutput: nil,
		},
		{
			location:       &sarif.Location{PhysicalLocation: &sarif.PhysicalLocation{}},
			expectedOutput: nil,
		},
		{
			location: CreateDummyLocation("filename", 1, 2, 3, 4, "snippet"),
			expectedOutput: sarif.NewRegion().WithStartLine(1).WithStartColumn(2).WithEndLine(3).WithEndColumn(4).
				WithSnippet(sarif.NewArtifactContent().WithText("snippet")),
		},
	}

	for _, test := range tests {
		assert.Equal(t, test.expectedOutput, getLocationRegion(test.location))
	}
}

func TestGetLocationStartLine(t *testing.T) {
	tests := []struct {
		location       *sarif.Location
		expectedOutput int
	}{
		{
			location:       nil,
			expectedOutput: 0,
		},
		{
			location:       CreateDummyLocation("filename", 1, 2, 3, 4, "snippet"),
			expectedOutput: 1,
		},
	}

	for _, test := range tests {
		assert.Equal(t, test.expectedOutput, GetLocationStartLine(test.location))
	}
}

func TestGetLocationStartColumn(t *testing.T) {
	tests := []struct {
		location       *sarif.Location
		expectedOutput int
	}{
		{
			location:       nil,
			expectedOutput: 0,
		},
		{
			location:       CreateDummyLocation("filename", 1, 2, 3, 4, "snippet"),
			expectedOutput: 2,
		},
	}

	for _, test := range tests {
		assert.Equal(t, test.expectedOutput, GetLocationStartColumn(test.location))
	}
}

func TestGetLocationEndLine(t *testing.T) {
	tests := []struct {
		location       *sarif.Location
		expectedOutput int
	}{
		{
			location:       nil,
			expectedOutput: 0,
		},
		{
			location:       CreateDummyLocation("filename", 1, 2, 3, 4, "snippet"),
			expectedOutput: 3,
		},
	}

	for _, test := range tests {
		assert.Equal(t, test.expectedOutput, GetLocationEndLine(test.location))
	}
}

func TestGetLocationEndColumn(t *testing.T) {
	tests := []struct {
		location       *sarif.Location
		expectedOutput int
	}{
		{
			location:       nil,
			expectedOutput: 0,
		},
		{
			location:       CreateDummyLocation("filename", 1, 2, 3, 4, "snippet"),
			expectedOutput: 4,
		},
	}

	for _, test := range tests {
		assert.Equal(t, test.expectedOutput, GetLocationEndColumn(test.location))
	}
}

func TestExtractRelativePath(t *testing.T) {
	tests := []struct {
		fullPath       string
		projectPath    string
		expectedResult string
	}{
		{fullPath: "file:///Users/user/Desktop/secrets_scanner/tests/req.nodejs/file.js",
			projectPath: "Users/user/Desktop/secrets_scanner/", expectedResult: "tests/req.nodejs/file.js"},
		{fullPath: "file:///private/Users/user/Desktop/secrets_scanner/tests/req.nodejs/file.js",
			projectPath: "Users/user/Desktop/secrets_scanner/", expectedResult: "tests/req.nodejs/file.js"},
		{fullPath: "invalidFullPath",
			projectPath: "Users/user/Desktop/secrets_scanner/", expectedResult: "invalidFullPath"},
		{fullPath: "",
			projectPath: "Users/user/Desktop/secrets_scanner/", expectedResult: ""},
		{fullPath: "file:///Users/user/Desktop/secrets_scanner/tests/req.nodejs/file.js",
			projectPath: "invalidProjectPath", expectedResult: "Users/user/Desktop/secrets_scanner/tests/req.nodejs/file.js"},
		{fullPath: "file:///private/Users/user/Desktop/secrets_scanner/tests/req.nodejs/file.js",
			projectPath: "invalidProjectPath", expectedResult: "Users/user/Desktop/secrets_scanner/tests/req.nodejs/file.js"},
	}

	for _, test := range tests {
		assert.Equal(t, test.expectedResult, ExtractRelativePath(test.fullPath, test.projectPath))
	}
}

func TestGetResultSeverity(t *testing.T) {
	levelValueHigh := string(errorLevel)
	levelValueMedium := string(warningLevel)
	levelValueMedium2 := string(infoLevel)
	levelValueLow := string(noteLevel)
	levelValueUnknown := string(noneLevel)

	tests := []struct {
		result           *sarif.Result
		expectedSeverity string
	}{
		{result: &sarif.Result{},
			expectedSeverity: "Medium"},
		{result: &sarif.Result{Level: &levelValueHigh},
			expectedSeverity: "High"},
		{result: &sarif.Result{Level: &levelValueMedium},
			expectedSeverity: "Medium"},
		{result: &sarif.Result{Level: &levelValueMedium2},
			expectedSeverity: "Medium"},
		{result: &sarif.Result{Level: &levelValueLow},
			expectedSeverity: "Low"},
		{result: &sarif.Result{Level: &levelValueUnknown},
			expectedSeverity: "Unknown"},
	}

	for _, test := range tests {
		assert.Equal(t, test.expectedSeverity, GetResultSeverity(test.result))
	}
}

func TestConvertToSarifLevel(t *testing.T) {
	tests := []struct {
		severity       string
		expectedOutput string
	}{
		{
			severity:       "Unknown",
			expectedOutput: "none",
		},
		{
			severity:       "Low",
			expectedOutput: "note",
		},
		{
			severity:       "Medium",
			expectedOutput: "warning",
		},
		{
			severity:       "High",
			expectedOutput: "error",
		},
		{
			severity:       "Critical",
			expectedOutput: "error",
		},
	}

	for _, test := range tests {
		assert.Equal(t, test.expectedOutput, ConvertToSarifLevel(test.severity))
	}
}

func TestIsApplicableResult(t *testing.T) {
	tests := []struct {
		name           string
		sarifResult    *sarif.Result
		expectedOutput bool
	}{
		{
			sarifResult:    CreateDummyPassingResult("rule"),
			expectedOutput: false,
		},
		{
			sarifResult:    CreateDummyResultWithOneLocation("file", 0, 0, 0, 0, "snippet1", "ruleId1", "level1"),
			expectedOutput: true,
		},
	}

	for _, test := range tests {
		assert.Equal(t, test.expectedOutput, IsApplicableResult(test.sarifResult))
	}
}

func TestGetRuleFullDescription(t *testing.T) {
	tests := []struct {
		rule           *sarif.ReportingDescriptor
		expectedOutput string
	}{
		{
			rule:           sarif.NewRule("rule"),
			expectedOutput: "",
		},
		{
			rule:           sarif.NewRule("rule").WithFullDescription(nil),
			expectedOutput: "",
		},
		{
			rule:           sarif.NewRule("rule").WithFullDescription(sarif.NewMultiformatMessageString("description")),
			expectedOutput: "description",
		},
	}

	for _, test := range tests {
		assert.Equal(t, test.expectedOutput, GetRuleFullDescription(test.rule))
	}
}

func TestCveToApplicabilityRuleId(t *testing.T) {
	assert.Equal(t, "applic_cve", CveToApplicabilityRuleId("cve"))
}

func TestApplicabilityRuleIdToCve(t *testing.T) {
	tests := []struct {
		ruleId         string
		expectedOutput string
	}{
		{
			ruleId:         "rule",
			expectedOutput: "rule",
		},
		{
			ruleId:         "applic_cve",
			expectedOutput: "cve",
		},
	}

	for _, test := range tests {
		assert.Equal(t, test.expectedOutput, ApplicabilityRuleIdToCve(test.ruleId))
	}
}

func TestGetRunRules(t *testing.T) {
	tests := []struct {
		run            *sarif.Run
		expectedOutput []*sarif.ReportingDescriptor
	}{
		{
			run:            &sarif.Run{},
			expectedOutput: []*sarif.ReportingDescriptor{},
		},
		{
			run:            CreateRunWithDummyResults(),
			expectedOutput: []*sarif.ReportingDescriptor{},
		},
		{
			run: CreateRunWithDummyResults(
				CreateDummyPassingResult("rule1"),
			),
			expectedOutput: []*sarif.ReportingDescriptor{sarif.NewRule("rule1")},
		},
		{
			run: CreateRunWithDummyResults(
				CreateDummyPassingResult("rule1"),
				CreateDummyPassingResult("rule1"),
				CreateDummyPassingResult("rule2"),
				CreateDummyPassingResult("rule3"),
				CreateDummyPassingResult("rule2"),
			),
			expectedOutput: []*sarif.ReportingDescriptor{sarif.NewRule("rule1"), sarif.NewRule("rule2"), sarif.NewRule("rule3")},
		},
	}

	for _, test := range tests {
		assert.Equal(t, test.expectedOutput, GetRunRules(test.run))
	}
}

func TestGetInvocationWorkingDirectory(t *testing.T) {
	tests := []struct {
		invocation     *sarif.Invocation
		expectedOutput string
	}{
		{
			invocation:     nil,
			expectedOutput: "",
		},
		{
			invocation:     sarif.NewInvocation(),
			expectedOutput: "",
		},
		{
			invocation:     sarif.NewInvocation().WithWorkingDirectory(nil),
			expectedOutput: "",
		},
		{
			invocation:     sarif.NewInvocation().WithWorkingDirectory(sarif.NewArtifactLocation()),
			expectedOutput: "",
		},
		{
			invocation:     sarif.NewInvocation().WithWorkingDirectory(sarif.NewArtifactLocation().WithUri("file_to_wd")),
			expectedOutput: "file_to_wd",
		},
	}

	for _, test := range tests {
		assert.Equal(t, test.expectedOutput, GetInvocationWorkingDirectory(test.invocation))
	}
}
