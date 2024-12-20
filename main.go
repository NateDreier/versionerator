package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sort"

	"github.com/google/go-github/github"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/zclconf/go-cty/cty"
	"github.com/blang/semver/v4"
)

type VersionInfo struct {
	Name string
	Version []string
	GithubLatest []string
	Source string
	NeedsUpgrade bool
}

type ProviderInfo struct {
	Providers []VersionInfo
}

type TerraformVersion struct {
	Version string
	Directory string
	GithubLatest string
	NeedsUpgrade bool
}

type TerraformVersions struct {
	TerraformVersion []TerraformVersion
}

func main() {
	terraformVersions := TerraformVersions{}
	// providers := ProviderInfo{}
	terraformDirectory := ""


	for _, path := range getProvidersFileList(getPath(terraformDirectory)) {
		if err := buildTfVersions(path, &terraformVersions); err != nil {
			return
		}

		// if err := buildVersionMap(path, &providers); err != nil {
		// 	return
		// }
	}

	var githubTagNames []string
	tags := getLatestTag("hashicorp", "terraform")
	for _, tag := range tags {
		t := *tag.Name
		if t[:1] == "v" {
			githubTagNames = append(githubTagNames, t[1:])
		} else {
			githubTagNames = append(githubTagNames, t)
		}
	}
	githubLatest := returnLatestSemvers(githubTagNames)
	for i, tfVersion := range terraformVersions.TerraformVersion {
		if len(tfVersion.Version) < 1 {
			continue
		}
		terraformVersions.TerraformVersion[i].GithubLatest = githubLatest[0]
		terraformVersions.TerraformVersion[i].NeedsUpgrade = compareTFSemvers(terraformVersions.TerraformVersion[i])
	}
	fmt.Println(terraformVersions)

	// fmt.Println(terraformVersions)

	// for i, provider := range providers.Providers {
    //     var githubTagNames []string
	// 	if len(provider.Version) < 1 {
	// 		continue
	// 	}
	// 	if provider.Source == "" {
	// 		continue
	// 	}
	// 	orgRepo := strings.Split(provider.Source, "/")
	// 	tags := getLatestTag(strings.Join(orgRepo[:len(orgRepo)-1], "/"), "terraform-provider-" + orgRepo[len(orgRepo)-1])
	// 	for _, tag := range tags {
	// 		t := *tag.Name
	// 		if t[:1] == "v" {
	// 			githubTagNames = append(githubTagNames, t[1:])
	// 		} else {
	// 			githubTagNames = append(githubTagNames, t)
	// 		}
	// 	}
	// 	providers.Providers[i] = VersionInfo{
	// 		Name: provider.Name,
	// 		Version: returnLatestSemvers(provider.Version),
	// 		GithubLatest: returnLatestSemvers(githubTagNames),
	// 		Source: provider.Source,
	// 	}
	// 	providers.Providers[i].NeedsUpgrade = compareSemvers(providers.Providers[i])
	// }
}

func compareTFSemvers(tfVersion TerraformVersion) bool {
	version1, err := semver.Parse(tfVersion.GithubLatest)
    if err != nil {
        fmt.Printf("Error parsing version: %s", err)
    }

    version2, err := semver.Parse(tfVersion.Version)
    if err != nil {
        fmt.Printf("Error parsing version: %s", err)
    }
    if version1.GT(version2) {
		return true
    }
	return false
}

func appendOrMergeTerraformVersionInfo(tfVersions *TerraformVersions, newVersionInfo TerraformVersion) {
	tfVersions.TerraformVersion = append(tfVersions.TerraformVersion, TerraformVersion{
			Version: newVersionInfo.Version,
			Directory: newVersionInfo.Directory,
		},
	)
}

func buildTfVersions(path string, tfVersions *TerraformVersions) error {
	body, err := getHCLBody(path)
	if err != nil {
		fmt.Printf("%s", err)
	}
	terraformVersion := getTerraformVersions(body)
	if terraformVersion == "" {
		return nil
	}
	newVersionInfo := TerraformVersion{
		Version: terraformVersion,
		Directory: path,
	}
	appendOrMergeTerraformVersionInfo(tfVersions, newVersionInfo)
	return nil
}

func compareSemvers(provider VersionInfo) bool {
	version1, err := semver.Parse(provider.GithubLatest[0])
    if err != nil {
        fmt.Printf("Error parsing version: %s", err)
    }

    version2, err := semver.Parse(provider.Version[0])
    if err != nil {
        fmt.Printf("Error parsing version: %s", err)
    }
    if version1.GT(version2) {
		return true
    }
	return false
}

func getTerraformVersions(body *hclsyntax.Body) string{
	var requiredVersion string
    for _, block := range body.Blocks {
        if block.Type == "terraform" {
			for _, attr := range block.Body.Attributes {
				if attr.Name  == "required_version" {
					value, diags := attr.Expr.Value(&hcl.EvalContext{})
					if diags.HasErrors() {
						fmt.Printf("%s", diags.Error())
					}
					if value.Type() == cty.String {
                        requiredVersion = value.AsString()
						versionSlice := strings.Split(requiredVersion, " ")
						if len(versionSlice) > 1 {
							return versionSlice[len(versionSlice)-1]
						}
						return versionSlice[1]
                    }
				}
			}
		}
	}
	return ""
}

func returnLatestSemvers(versionsSlice []string) []string {
	slices.Sort(versionsSlice)
	return []string{sortSemver(versionsSlice)}
}

func getProviderVersions(body *hclsyntax.Body) VersionInfo {
	var info VersionInfo
    // Iterate over the blocks in the file
    for _, block := range body.Blocks {
        if block.Type == "terraform" {
            for _, terraformBlock := range block.Body.Blocks {
                if terraformBlock.Type == "required_providers" {
                    providerAttributes := terraformBlock.Body.Attributes
                    for name, attr := range providerAttributes {
                        value, _ := attr.Expr.Value(&hcl.EvalContext{
                            Variables: map[string]cty.Value{},
                        })

                        if value.Type().IsObjectType() {
                            providerMap := value.AsValueMap()
							version := providerMap["version"].AsString()
							source := providerMap["source"].AsString()
							if 1 < len(strings.Fields(version)) {
								version = strings.Fields(version)[1]
							} else {
								version = strings.Fields(version)[0]
							}
							info = VersionInfo{
								Name: name,
								Version: []string{version},
								Source: source,
							}
                        }
                    }
                }
            }
        }
    }
	return info
}

func getHCLBody(path string) (*hclsyntax.Body, error) {
    // Parse the file
    parser := hclparse.NewParser()
    hclFile, diags := parser.ParseHCLFile(path)
    if diags.HasErrors() {
		return nil, diags.Errs()[0]
    }

    // Extract the body from the parsed file
    return hclFile.Body.(*hclsyntax.Body), nil
}

func sortSemver(versions []string) string {
    var semvers []semver.Version
    for _, v := range versions {
        parsed, err := semver.Parse(v)
        if err != nil {
            fmt.Printf("Error parsing version %s: %s", v, err)
        }
		p := stripPreReleaseAndBuild(parsed)
        semvers = append(semvers, p)
    }

    sort.Slice(semvers, func(i, j int) bool {
        return semvers[i].LT(semvers[j])
    })

    sortedVersions := make([]string, len(semvers))
    for i, v := range semvers {
        sortedVersions[i] = v.String()
    }

	return sortedVersions[len(sortedVersions)-1]
}

func stripPreReleaseAndBuild(v semver.Version) semver.Version {
    v.Pre = nil
    v.Build = nil
	return v
}

func getLatestTag(org string, repo string) []*github.RepositoryTag {
	client := github.NewClient(nil)
	ctx := context.Background()

	tags, _, err := client.Repositories.ListTags(ctx, org, repo, &github.ListOptions{})
	if err != nil {
		fmt.Printf("%s", err)
	}
	return tags
}

func getPath(terraformDirectory string) string {
	var builder strings.Builder

	// get home dir
	homedir, err := os.UserHomeDir()
	if err != nil {
        fmt.Printf("%s", err)
    }

	builder.WriteString(homedir)
	builder.WriteString(terraformDirectory)
	return builder.String()
}

func getProvidersFileList(absoluteTerraformPath string) []string {
	var fileList []string

	if err := filepath.Walk(absoluteTerraformPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && info.Name() == "providers.tf" {
			fileList = append(fileList, path)
		}
		return nil
	}); err != nil {
		fmt.Printf("%s", err)
	}
	return fileList
}

func buildVersionMap(path string, providers *ProviderInfo) error {
	body, err := getHCLBody(path)
	if err != nil {
		return err
	}

	appendOrMergeProviderInfo(providers, getProviderVersions(body))

	return nil
}

func appendOrMergeProviderInfo(providers *ProviderInfo, newVersionInfo VersionInfo) {
	merged := false
	for i, versionInfo := range providers.Providers {
		if versionInfo.Name == newVersionInfo.Name {
			providers.Providers[i] = VersionInfo{
				Name: versionInfo.Name,
				Version: append(versionInfo.Version, newVersionInfo.Version...),
				Source: versionInfo.Source,
			}
			merged = true
		}
	}
	if !merged {
		providers.Providers = append(providers.Providers, newVersionInfo)
	}
}
