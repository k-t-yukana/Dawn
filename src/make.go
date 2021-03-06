// +build ignore

/**
 * Inspired by:
 * https://github.com/camlistore/camlistore/blob/master/make.go
 */

package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	yaml "gopkg.in/yaml.v2"

	"flag"
)

type BuildConfig struct {
	Homepage      string              `yaml:"homepage"`
	GitHub        GitHubBuildConfig   `yaml:"github"`
	Configuration ConfigurationConfig `yaml:"configuration"`
	Binary        BinaryBuildConfig   `yaml:"binary"`
	Image         ImageBuildConfig    `yaml:"image"`
}

type ConfigurationConfig struct {
	Folder   string `yaml:"folder"`
	Filename string `yaml:"filename"`
}

type GitHubBuildConfig struct {
	Organization string `yaml:"organization"`
	Name         string `yaml:"name"`
}

type BinaryBuildConfig struct {
	Name             string                 `yaml:"name"`
	Version          string                 `yaml:"version"`
	DocumentationURL string                 `yaml:"documentation_url"`
	InstallURLs      InstallURLsBuildConfig `yaml:"install_urls,omitempty"`
}

type InstallURLsBuildConfig struct {
	Windows string `yaml:"windows,omitempty"`
	Others  string `yaml:"others,omitempty"`
}

type ImageBuildConfig struct {
	Organization string `yaml:"organization"`
	Name         string `yaml:"name"`
	Version      string `yaml:"version"`
	RootFolder   string `yaml:"root_folder"`
	ShellUser    string `yaml:"shell_user"`
}

type Step func()

var (
	target  string
	version string
	image   string
)

func main() {
	targetPtr := flag.String("target", runtime.GOOS, "Operating system to build for. (windows, linux, darwin or all)")
	versionPtr := flag.String("version", "development", "Version number to attach to this build)")
	imagePtr := flag.String("image", "development", "Default image to use when creating new environments)")

	flag.Parse()

	target = *targetPtr
	version = *versionPtr
	image = *imagePtr

	if target == "all" {
		target = "darwin"
		runStep("Building binary (darwin)", build)

		target = "windows"
		runStep("Building binary (windows)", build)

		target = "linux"
		runStep("Building binary (linux)", build)
	} else {
		runStep("Building binary", build)
	}
}

func getBuildConfiguration() (*BuildConfig, error) {
	var buildConfig BuildConfig

	data, err := ioutil.ReadFile("../buildconfig.yml")
	if err != nil {
		return nil, err
	}

	err = yaml.Unmarshal(data, &buildConfig)
	if err != nil {
		return nil, err
	}

	if buildConfig.Binary.InstallURLs.Windows == "" {
		buildConfig.Binary.InstallURLs.Windows = fmt.Sprintf(
			"https://github.com/%s/%s/blob/develop/scripts/install/install.ps1",
			buildConfig.GitHub.Organization,
			buildConfig.GitHub.Name)
	}

	if buildConfig.Binary.InstallURLs.Others == "" {
		buildConfig.Binary.InstallURLs.Others = fmt.Sprintf(
			"https://github.com/%s/%s/blob/develop/scripts/install/install.sh",
			buildConfig.GitHub.Organization,
			buildConfig.GitHub.Name)
	}

	if buildConfig.Binary.DocumentationURL == "" {
		buildConfig.Binary.DocumentationURL = fmt.Sprintf(
			"https://%s.github.io/%s",
			strings.ToLower(buildConfig.GitHub.Organization),
			strings.ToLower(buildConfig.GitHub.Name))
	}

	return &buildConfig, nil
}

func runSubProcess(command string, arguments []string) {
	cmd := exec.Command(command, arguments...)

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	cmd.Env = append(os.Environ(), fmt.Sprintf("GOOS=%s", target))

	err := cmd.Run()

	if err != nil {
		log.Fatalf("Failed to run command: %#v", err)
	}
}

func runStep(description string, step Step) {
	fmt.Printf("\n[make] %s [make]\n\n", description)
	step()
}

func getCommitHash() string {
	cmd := exec.Command("git", "rev-parse", "--verify", "HEAD")
	output, err := cmd.Output()

	if err != nil {
		log.Fatalf("Failed to retrieve git commit hash: %v", err)
	}

	return string(output)
}

func build() {
	buildConfiguration, err := getBuildConfiguration()

	if err != nil {
		log.Fatalf("Failed to load build configuration: %#v", err)
	}

	targetPath := fmt.Sprintf("./dist/%s", target)
	bin := buildConfiguration.Binary.Name

	if target == "windows" {
		bin = fmt.Sprintf("%s.exe", bin)
	}

	hostname, _ := os.Hostname()
	now := time.Now()
	commitHash := getCommitHash()

	ldFlags := []string{
		fmt.Sprintf("-X main.cliName=%s", buildConfiguration.Binary.Name),
		fmt.Sprintf("-X main.cliVersion=%s", version),

		fmt.Sprintf("-X main.cliConfigurationFolder=%s", buildConfiguration.Configuration.Folder),
		fmt.Sprintf("-X main.cliConfigurationFilename=%s", buildConfiguration.Configuration.Filename),

		fmt.Sprintf("-X main.cliWindowsInstallURL=%s", buildConfiguration.Binary.InstallURLs.Windows),
		fmt.Sprintf("-X main.cliOthersInstallURL=%s", buildConfiguration.Binary.InstallURLs.Others),
		fmt.Sprintf("-X main.cliDocsURL=%s", buildConfiguration.Binary.DocumentationURL),

		fmt.Sprintf("-X main.cliDefaultImageName=%s/%s", buildConfiguration.Image.Organization, buildConfiguration.Image.Name),
		fmt.Sprintf("-X main.cliDefaultImageVersion=%s", image),
		fmt.Sprintf("-X main.cliRootFolder=%s", buildConfiguration.Image.RootFolder),
		fmt.Sprintf("-X main.cliShellUser=%s", buildConfiguration.Image.ShellUser),

		fmt.Sprintf("-X main.cliCommitHash=%s", commitHash),
		fmt.Sprintf("-X main.cliBuildTime=%s", now.Format(time.RFC3339)),
		fmt.Sprintf("-X main.cliBuildServer=%s", hostname),
	}

	os.MkdirAll(targetPath, 0700)

	args := []string{
		"build",
		"-ldflags",
		strings.Join(ldFlags, " "),
		"-o",
		fmt.Sprintf("%s/%s", targetPath, bin),
		"cli.go",
	}

	runSubProcess("go", args)
}
