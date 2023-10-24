package coreutils

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/exp/maps"
)

func TestDetectTechnologiesByFilePaths(t *testing.T) {
	tests := []struct {
		name     string
		paths    []string
		expected map[Technology]bool
	}{
		{"simpleMavenTest", []string{"pom.xml"}, map[Technology]bool{Maven: true}},
		{"npmTest", []string{"../package.json"}, map[Technology]bool{Npm: true}},
		{"yarnTest", []string{"./package.json", "./.yarn"}, map[Technology]bool{Yarn: true}},
		{"windowsGradleTest", []string{"c:\\users\\test\\package\\build.gradle"}, map[Technology]bool{Gradle: true}},
		{"windowsPipTest", []string{"c:\\users\\test\\package\\setup.py"}, map[Technology]bool{Pip: true}},
		{"windowsPipenvTest", []string{"c:\\users\\test\\package\\Pipfile"}, map[Technology]bool{Pipenv: true}},
		{"golangTest", []string{"/Users/eco/dev/jfrog-cli-core/go.mod"}, map[Technology]bool{Go: true}},
		{"windowsNugetTest", []string{"c:\\users\\test\\package\\project.sln"}, map[Technology]bool{Nuget: true, Dotnet: true}},
		{"noTechTest", []string{"pomxml"}, map[Technology]bool{}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			detectedTech := detectTechnologiesByFilePaths(test.paths, false)
			assert.True(t, reflect.DeepEqual(test.expected, detectedTech), "expected: %s, actual: %s", test.expected, detectedTech)
		})
	}
}

func TestMapFilesToRelevantWorkingDirectories(t *testing.T) {
	noRequest := map[Technology][]string{}
	noExclude := map[string][]Technology{}

	tests := []struct {
		name                 string
		paths                []string
		requestedDescriptors map[Technology][]string
		expectedWorkingDir   map[string][]string
		expectedExcluded     map[string][]Technology
	}{
		{
			name:                 "noTechTest",
			paths:                []string{"pomxml", filepath.Join("sub1", "file"), filepath.Join("sub", "sub", "file")},
			requestedDescriptors: noRequest,
			expectedWorkingDir:   map[string][]string{},
			expectedExcluded:     noExclude,
		},
		{
			name:                 "mavenTest",
			paths:                []string{"pom.xml", filepath.Join("sub1", "pom.xml"), filepath.Join("sub2", "pom.xml")},
			requestedDescriptors: noRequest,
			expectedWorkingDir: map[string][]string{
				".":    {"pom.xml"},
				"sub1": {filepath.Join("sub1", "pom.xml")},
				"sub2": {filepath.Join("sub2", "pom.xml")},
			},
			expectedExcluded: noExclude,
		},
		{
			name:                 "npmTest",
			paths:                []string{filepath.Join("dir", "package.json"), filepath.Join("dir", "package-lock.json"), filepath.Join("dir2", "npm-shrinkwrap.json")},
			requestedDescriptors: noRequest,
			expectedWorkingDir: map[string][]string{
				"dir":  {filepath.Join("dir", "package.json"), filepath.Join("dir", "package-lock.json")},
				"dir2": {filepath.Join("dir2", "npm-shrinkwrap.json")},
			},
			expectedExcluded: noExclude,
		},
		{
			name:                 "yarnTest",
			paths:                []string{filepath.Join("dir", "package.json"), filepath.Join("dir", ".yarn")},
			requestedDescriptors: noRequest,
			expectedWorkingDir:   map[string][]string{"dir": {filepath.Join("dir", "package.json"), filepath.Join("dir", ".yarn")}},
			expectedExcluded:     map[string][]Technology{"dir": {Npm}},
		},
		{
			name:                 "golangTest",
			paths:                []string{filepath.Join("dir", "dir2", "go.mod")},
			requestedDescriptors: noRequest,
			expectedWorkingDir:   map[string][]string{filepath.Join("dir", "dir2"): {filepath.Join("dir", "dir2", "go.mod")}},
			expectedExcluded:     noExclude,
		},
		{
			name: "pipTest",
			paths: []string{
				filepath.Join("users_dir", "test", "package", "setup.py"),
				filepath.Join("users_dir", "test", "package", "blabla.txt"),
				filepath.Join("users_dir", "test", "package2", "requirements.txt"),
			},
			requestedDescriptors: noRequest,
			expectedWorkingDir: map[string][]string{
				filepath.Join("users_dir", "test", "package"):  {filepath.Join("users_dir", "test", "package", "setup.py")},
				filepath.Join("users_dir", "test", "package2"): {filepath.Join("users_dir", "test", "package2", "requirements.txt")}},
			expectedExcluded: noExclude,
		},
		{
			name:                 "pipRequestedDescriptorTest",
			paths:                []string{filepath.Join("dir", "blabla.txt"), filepath.Join("dir", "somefile")},
			requestedDescriptors: map[Technology][]string{Pip: {"blabla.txt"}},
			expectedWorkingDir:   map[string][]string{"dir": {filepath.Join("dir", "blabla.txt")}},
			expectedExcluded:     noExclude,
		},
		{
			name:                 "pipenvTest",
			paths:                []string{filepath.Join("users", "test", "package", "Pipfile")},
			requestedDescriptors: noRequest,
			expectedWorkingDir:   map[string][]string{filepath.Join("users", "test", "package"): {filepath.Join("users", "test", "package", "Pipfile")}},
			expectedExcluded:     map[string][]Technology{filepath.Join("users", "test", "package"): {Pip}},
		},
		{
			name:                 "gradleTest",
			paths:                []string{filepath.Join("users", "test", "package", "build.gradle"), filepath.Join("dir", "build.gradle.kts"), filepath.Join("dir", "file")},
			requestedDescriptors: noRequest,
			expectedWorkingDir: map[string][]string{
				filepath.Join("users", "test", "package"): {filepath.Join("users", "test", "package", "build.gradle")},
				"dir": {filepath.Join("dir", "build.gradle.kts")},
			},
			expectedExcluded: noExclude,
		},
		{
			name:                 "nugetTest",
			paths:                []string{filepath.Join("dir", "project.sln"), filepath.Join("dir", "sub1", "project.csproj"), filepath.Join("dir", "file")},
			requestedDescriptors: noRequest,
			expectedWorkingDir: map[string][]string{
				"dir":                        {filepath.Join("dir", "project.sln")},
				filepath.Join("dir", "sub1"): {filepath.Join("dir", "sub1", "project.csproj")},
			},
			expectedExcluded: noExclude,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			detectedWd, detectedExcluded := mapFilesToRelevantWorkingDirectories(test.paths, test.requestedDescriptors)
			expectedKeys := maps.Keys(test.expectedWorkingDir)
			actualKeys := maps.Keys(detectedWd)
			assert.ElementsMatch(t, expectedKeys, actualKeys, "expected: %s, actual: %s", expectedKeys, actualKeys)
			for key, value := range test.expectedWorkingDir {
				assert.ElementsMatch(t, value, detectedWd[key], "expected: %s, actual: %s", value, detectedWd[key])
			}
			assert.True(t, reflect.DeepEqual(test.expectedExcluded, detectedExcluded), "expected: %s, actual: %s", test.expectedExcluded, detectedExcluded)
		})
	}
}

func TestMapWorkingDirectoriesToTechnologies(t *testing.T) {
	noRequestSpecialDescriptors := map[Technology][]string{}
	noRequestTech := []Technology{}
	// noExclude := map[string][]Technology{}
	tests := []struct {
		name                         string
		workingDirectoryToIndicators map[string][]string
		excludedTechAtWorkingDir     map[string][]Technology
		requestedTechs               []Technology
		requestedDescriptors         map[Technology][]string

		expected map[Technology]map[string][]string
	}{
		// {
		// 	name: "noTechTest",
		// 	workingDirectoryToIndicators: map[string][]string{},
		// 	excludedTechAtWorkingDir: noExclude,
		// 	requestedTechs: noRequestTech,
		// 	requestedDescriptors: noRequestSpecialDescriptors,
		// 	expected: map[Technology]map[string][]string{},
		// },
		{
			name: "all techs test",
			workingDirectoryToIndicators: map[string][]string{
				"folder":                        {filepath.Join("folder", "pom.xml")},
				filepath.Join("folder", "sub1"): {filepath.Join("folder", "sub1", "pom.xml")},
				filepath.Join("folder", "sub2"): {filepath.Join("folder", "sub2", "pom.xml")},
				"dir":                           {filepath.Join("dir", "package.json"), filepath.Join("dir", "package-lock.json"), filepath.Join("dir", "build.gradle.kts"), filepath.Join("dir", "project.sln")},
				"directory":                     {filepath.Join("directory", "npm-shrinkwrap.json")},
				"dir3":                          {filepath.Join("dir3", "package.json"), filepath.Join("dir3", ".yarn")},
				filepath.Join("dir", "dir2"):    {filepath.Join("dir", "dir2", "go.mod")},
				filepath.Join("users_dir", "test", "package"):  {filepath.Join("users_dir", "test", "package", "setup.py")},
				filepath.Join("users_dir", "test", "package2"): {filepath.Join("users_dir", "test", "package2", "requirements.txt")},
				filepath.Join("users", "test", "package"):      {filepath.Join("users", "test", "package", "Pipfile"), filepath.Join("users", "test", "package", "build.gradle")},
				filepath.Join("dir", "sub1"):                   {filepath.Join("dir", "sub1", "project.csproj")},
			},
			excludedTechAtWorkingDir: map[string][]Technology{
				filepath.Join("users", "test", "package"): {Pip},
				"dir3": {Npm},
			},
			requestedTechs:       noRequestTech,
			requestedDescriptors: noRequestSpecialDescriptors,
			expected: map[Technology]map[string][]string{
				Maven: {"folder": {filepath.Join("folder", "pom.xml"), filepath.Join("folder", "sub1", "pom.xml"), filepath.Join("folder", "sub2", "pom.xml")}},
				Npm: {
					"dir":       {filepath.Join("dir", "package.json")},
					"directory": {},
				},
				Yarn: {"dir3": {filepath.Join("dir3", "package.json")}},
				Go:   {filepath.Join("dir", "dir2"): {filepath.Join("dir", "dir2", "go.mod")}},
				Pip: {
					filepath.Join("users_dir", "test", "package"):  {filepath.Join("users_dir", "test", "package", "setup.py")},
					filepath.Join("users_dir", "test", "package2"): {filepath.Join("users_dir", "test", "package2", "requirements.txt")},
				},
				Pipenv: {filepath.Join("users", "test", "package"): {filepath.Join("users", "test", "package", "Pipfile")}},
				Gradle: {
					"dir": {filepath.Join("dir", "build.gradle.kts")},
					filepath.Join("users", "test", "package"): {filepath.Join("users", "test", "package", "build.gradle")},
				},
				Nuget:  {"dir": {filepath.Join("dir", "project.sln"), filepath.Join("dir", "sub1", "project.csproj")}},
				Dotnet: {"dir": {filepath.Join("dir", "project.sln"), filepath.Join("dir", "sub1", "project.csproj")}},
			},
		},
		// {"noTechTest", map[string][]string{}, map[string][]Technology{}, []Technology{}, map[Technology][]string{}, map[Technology]map[string][]string{}},
		// {"mavenTest", map[string][]string{".": {"dir/pom.xml"}, "sub1": {"dir/sub1/pom.xml"}, "sub2": {"dir/sub2/pom.xml"}}, map[string][]Technology{}, []Technology{Maven}, map[Technology][]string{}, map[Technology]map[string][]string{Maven: {"dir": {"dir/pom.xml","dir/sub1/pom.xml","dir/sub2/pom.xml"}}}},
		// {"npmTest", map[string][]string{"..": {"../package.json"}}, map[string][]Technology{}, []Technology{Npm}, map[Technology][]string{}, map[Technology]map[string][]string{Npm: {"..": {"../package.json"}}}},
		// {"yarnTest", map[string][]string{".": {"./package.json", "./.yarn"}}, map[string][]Technology{".": {Npm}}, []Technology{Yarn}, map[Technology][]string{}, map[Technology]map[string][]string{Yarn: {".": {"./package.json", "./.yarn"}}}},
		// {"golangTest", map[string][]string{"/Users/eco/dev/jfrog-cli-core": {"/Users/eco/dev/jfrog-cli-core/go.mod"}}, map[string][]Technology{}, []Technology{Go}, map[Technology][]string{}, map[Technology]map[string][]string{Go: {"/Users/eco/dev/jfrog-cli-core": {"/Users/eco/dev/jfrog-cli-core/go.mod"}}}},
		// {"pipRequestedDescriptorTest", map[string][]string{"dir": {"dir/blabla.txt"}}, map[string][]Technology{}, []Technology{Pip}, map[Technology][]string{Pip: {"blabla.txt"}}, map[Technology]map[string][]string{Pip: {"dir": {"dir/blabla.txt"}}}},
		// {"pipenvTest", map[string][]string{"c:\\users\\test\\package": {"c:\\users\\test\\package\\Pipfile"}}, map[string][]Technology{}, []Technology{Pipenv}, map[Technology][]string{}, map[Technology]map[string][]string{Pipenv: {"c:\\users\\test\\package": {"c:\\users\\test\\package\\Pipfile"}}}},
		// {"gradleTest", map[string][]string{"c:\\users\\test\\package": {"c:\\users\\test\\package\\build.gradle"}}, map[string][]Technology{}, []Technology{Gradle}, map[Technology][]string{}, map[Technology]map[string][]string{Gradle: {"c:\\users\\test\\package": {"c:\\users\\test\\package\\build.gradle"}}}},
		// {"nugetTest", map[string][]string{"c:\\users\\test\\package": {"c:\\users\\test\\package\\project.sln"}}, map[string][]Technology{}, []Technology{Nuget}, map[Technology][]string{}, map[Technology]map[string][]string{Nuget: {"c:\\users\\test\\package": {"c:\\users\\test\\package\\project.sln"}}}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			detectedTech := mapWorkingDirectoriesToTechnologies(test.workingDirectoryToIndicators, test.excludedTechAtWorkingDir, test.requestedTechs, test.requestedDescriptors)
			expectedKeys := maps.Keys(test.expected)
			detectedKeys := maps.Keys(detectedTech)
			assert.ElementsMatch(t, expectedKeys, detectedKeys, "expected: %s, actual: %s", expectedKeys, detectedKeys)
			for key, value := range test.expected {
				actualKeys := maps.Keys(detectedTech[key])
				expectedKeys := maps.Keys(value)
				assert.ElementsMatch(t, expectedKeys, actualKeys, "for tech %s, expected: %s, actual: %s", key, expectedKeys, actualKeys)
				// assert.ElementsMatch(t, value, actual[key], "expected: %s, actual: %s", value, actual[key])
				for innerKey, innerValue := range value {
					assert.ElementsMatch(t, innerValue, detectedTech[key][innerKey], "expected: %s, actual: %s", innerValue, detectedTech[key][innerKey])
				}
			}
			// assert.True(t, reflect.DeepEqual(test.expected, detectedTech), "expected: %s, actual: %s", test.expected, detectedTech)
		})
	}
}

func TestContainsApplicabilityScannableTech(t *testing.T) {
	tests := []struct {
		name         string
		technologies []Technology
		want         bool
	}{
		{name: "contains supported and unsupported techs", technologies: []Technology{Nuget, Go, Npm}, want: true},
		{name: "contains supported techs only", technologies: []Technology{Maven, Yarn, Npm}, want: true},
		{name: "contains unsupported techs only", technologies: []Technology{Dotnet, Nuget, Go}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ContainsApplicabilityScannableTech(tt.technologies))
		})
	}
}
