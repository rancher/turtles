/*
Copyright © 2023 - 2025 SUSE LLC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"flag"
	"fmt"
	"maps"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/agnivade/levenshtein"
	"github.com/spf13/pflag"

	"embed"
)

//go:embed applications/*
var Applications embed.FS

//go:embed clusterclasses/*
var ClusterClasses embed.FS

var (
	clusterClassList  bool
	clusterClassRegex string
)

// Examples represents a collection of example files and directories.
type Examples struct {
	// Applications is a list of file paths to application YAML/YML files.
	Applications map[string]string

	// ClusterClasses is a list of file paths to clusterclass YAML/YML files.
	ClusterClasses map[string]string

	// dirs is a list of directory paths to be processed.
	dirs []string
}

// Collect traverses the directory tree from root and collects all yaml/yml examples.
func (e *Examples) Collect() (err error) {
	e.Applications = map[string]string{}
	e.ClusterClasses = map[string]string{}

	e.dirs = append(e.dirs, ".")
	for len(e.dirs) > 0 {
		root := e.dirs[len(e.dirs)-1]
		e.dirs = e.dirs[:len(e.dirs)-1]

		apps, err := e.collectFiles(Applications, root, func(key string) string {
			key = strings.TrimPrefix(key, "applications/")
			return strings.ReplaceAll(key, string(filepath.Separator), "-")
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v", err)

			return err
		}

		maps.Insert(e.Applications, maps.All(apps))
	}

	e.dirs = append(e.dirs, ".")
	for len(e.dirs) > 0 {
		root := e.dirs[len(e.dirs)-1]
		e.dirs = e.dirs[:len(e.dirs)-1]

		classes, err := e.collectFiles(ClusterClasses, root, func(key string) string {
			key = strings.TrimPrefix(key, "clusterclasses/")
			return strings.ReplaceAll(key, string(filepath.Separator), "-")
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v", err)

			return err
		}

		maps.Insert(e.ClusterClasses, maps.All(classes))
	}

	return nil
}

// collectFiles collects all yaml/yml files and subdirectories from root.
func (e *Examples) collectFiles(fs embed.FS, root string, keyFn func(string) string) (map[string]string, error) {
	entries, err := fs.ReadDir(root)
	if err != nil {
		return nil, err
	}

	files := map[string]string{}
	for _, entry := range entries {
		path := filepath.Join(root, entry.Name())

		if entry.IsDir() {
			e.dirs = append(e.dirs, path)
			continue
		} else if !strings.HasSuffix(entry.Name(), ".yaml") && !strings.HasSuffix(entry.Name(), ".yml") {
			continue
		}

		content, err := fs.ReadFile(path)
		if err != nil {
			return nil, err
		}

		// Parent path as key and content as value
		path = keyFn(root)
		files[path] = joinYamls(files[path], string(content))
	}

	return files, nil
}

// joinYamls takes a list of yaml strings and joins them into a single yaml
func joinYamls(yamls ...string) string {
	joined := ""
	for _, yaml := range yamls {
		if yaml == "" {
			continue
		}

		joined += "\n---\n" + yaml
	}

	return joined
}

// ClusterClassRegex returns ClusterClasses matching the given search regex.
func (e *Examples) ClusterClassRegex(search *regexp.Regexp) (string, error) {
	keys := []string{}
	for k := range e.ClusterClasses {
		if search.MatchString(k) {
			keys = append(keys, k)
		}
	}

	if len(keys) == 0 {
		return "", fmt.Errorf("clusterclasses not found: %s, maybe you want to use: %s?\n", search, e.closestMatch(search.String()))
	} else {
		data := slices.Collect(maps.Values(e.Applications))
		for _, key := range keys {
			data = append(data, e.ClusterClasses[key])
		}

		return joinYamls(data...), nil
	}
}

// ClusterClass returns the ClusterClass specified by the given key.
func (e *Examples) ClusterClass(key string) (string, error) {
	if clusterClass, found := e.ClusterClasses[key]; !found {
		return "", fmt.Errorf("clusterclasses not found: %s, maybe you want to use: %s?\n", key, e.closestMatch(key))
	} else {
		data := slices.Collect(maps.Values(e.Applications))
		data = append(data, clusterClass)

		return joinYamls(data...), nil
	}
}

// closestMatch finds the closest matching key from the ClusterClasses map

func (e *Examples) closestMatch(key string) string {
	closestMatch, shortestDistance := "", math.MaxInt
	for k := range e.ClusterClasses {
		distance := levenshtein.ComputeDistance(k, key)
		if distance < shortestDistance {
			shortestDistance = distance
			closestMatch = k
		}
	}

	return closestMatch
}

// initFlags initializes the flags.
func initFlags(fs *pflag.FlagSet) {
	fs.BoolVarP(&clusterClassList, "list", "l", false,
		"List cluster class names from examples")

	fs.StringVarP(&clusterClassRegex, "regex", "r", "",
		"ClusterClass search regex")
}

func main() {
	initFlags(pflag.CommandLine)
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.Parse()

	examples := Examples{}
	if err := examples.Collect(); err != nil {
		fmt.Fprintf(os.Stderr, "%v", err)

		os.Exit(1)
	}

	if clusterClassList {
		keys := maps.Keys(examples.ClusterClasses)
		fmt.Printf("Available classes: %s", slices.Collect(keys))

		return
	}
	if clusterClassRegex != "" {
		data, err := examples.ClusterClassRegex(regexp.MustCompile(clusterClassRegex))
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v", err)

			os.Exit(1)
		}

		fmt.Printf("%s", data)

		return
	}

	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Please provide the search key for examples\n")
		os.Exit(1)
	}

	data, err := examples.ClusterClass(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v", err)

		os.Exit(1)
	}

	fmt.Printf("%s", data)
}
