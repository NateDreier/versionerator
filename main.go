package main

import (
	// "context"
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	// "github.com/google/go-github/github"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/zclconf/go-cty/cty"
	"github.com/rs/zerolog"
)



func main() {
	var builder strings.Builder
	logger := zerolog.New(os.Stdout)
	// client := github.NewClient(nil)
	// ctx := context.Background()

	// tags, _, err := client.Repositories.ListTags(ctx, "hashicorp", "terraform-provider-aws", &github.ListOptions{})
	// if err != nil {
	// 	logger.Error().Err(err).Msg("")
	// }
	// for _, tag := range(tags) {
	// 	fmt.Println(*tag.Name)
	// }

	// get home dir
	homedir, err := os.UserHomeDir()
	if err != nil {
        logger.Error().Err(err).Msg(err.Error())
    }

	builder.WriteString(homedir)
	builder.WriteString("")
	provider_versions := make(map[string]string)
	var tf_versions []string
	if err := filepath.Walk(builder.String(), func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && info.Name() == "providers.tf"{
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()

			scanner := bufio.NewScanner(file)
			lineNumber := 1
			searchString := "required_version"
			for scanner.Scan() {
				line := scanner.Text()
				if strings.Contains(line, searchString){
					re := regexp.MustCompile(`(v)?(\d+)\.(\d+)\.(\d+)`)
					match := re.FindStringSubmatch(line)
					tf_versions = append(tf_versions, match[0])
					// data, _ := os.ReadFile(path)
					parseHcl(path, provider_versions)
				}
				lineNumber++
			}
		}


		return nil
	}); err != nil {
		logger.Error().Err(err).Msg("")
	}

	slices.Sort(tf_versions)
	tf_versions = slices.Compact(tf_versions)
	fmt.Println(tf_versions)
	fmt.Println(provider_versions)
}


func parseHcl(path string, provider_versions map[string]string) {
    // Open the providers.tf file
    file, err := os.Open(path)
    if err != nil {
        fmt.Println("foo")
    }
    defer file.Close()

    // Parse the file
    parser := hclparse.NewParser()
    hclFile, diags := parser.ParseHCLFile(path)
    if diags.HasErrors() {
        fmt.Printf("Failed to parse providers.tf: %s", diags)
    }

    // Extract the body from the parsed file
    body := hclFile.Body.(*hclsyntax.Body)
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
                            // source := providerMap["source"].AsString()
                            version := providerMap["version"].AsString()
							if 1 < len(strings.Fields(version)) {
								version = strings.Fields(version)[1]
							} else {
								version = strings.Fields(version)[0]
							}
							provider_versions[name] = version
                        }
                    }
                }
            }
        }
    }
}
