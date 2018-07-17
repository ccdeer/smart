// Copyright 2017-18 Daniel Swarbrick. All rights reserved.
// Use of this source code is governed by a GPL license that can be found in the LICENSE file.

// Smartmontools drivedb.h database to YAML format converter.
//
package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"text/scanner"

	"gopkg.in/yaml.v2"
)

const (
	defaultDrivedbURL = "https://www.smartmontools.org/export/HEAD/trunk/smartmontools/drivedb.h"
)

type AttrConv struct {
	Conv string `yaml:"conv,omitempty"`
	Name string `yaml:"name,omitempty"`
}

type DriveModel struct {
	Family        string              `yaml:"family,omitempty"`
	ModelRegex    string              `yaml:"model_regex,omitempty"`
	FirmwareRegex string              `yaml:"firmware_regex,omitempty"`
	WarningMsg    string              `yaml:"warning,omitempty"`
	Presets       map[string]AttrConv `yaml:"presets,omitempty"`
}

type DriveDb struct {
	Drives []DriveModel `yaml:"drives"`
}

func parseDrivedb(src io.Reader) []DriveModel {
	var (
		s    scanner.Scanner
		prev rune
		idx  int
	)

	drives := make([]DriveModel, 0)
	items := make([]string, 5)

	s.Init(src)

	for tok := s.Scan(); tok != scanner.EOF; tok = s.Scan() {
		if (prev == '{' || prev == ',') && tok == scanner.String {
			items[idx] = strings.Trim(s.TokenText(), `"`)
		} else if prev == scanner.String && tok == ',' {
			idx++
		} else if prev == scanner.String && tok == scanner.String {
			items[idx] += strings.Trim(s.TokenText(), `"`)
		} else if tok == '}' {
			dm := DriveModel{Presets: make(map[string]AttrConv)}

			if tmp, err := strconv.Unquote(`"` + items[0] + `"`); err == nil {
				dm.Family = tmp
			}

			if tmp, err := strconv.Unquote(`"` + items[1] + `"`); err == nil {
				dm.ModelRegex = tmp
			}

			if tmp, err := strconv.Unquote(`"` + items[2] + `"`); err == nil {
				dm.FirmwareRegex = tmp
			}

			if tmp, err := strconv.Unquote(`"` + items[3] + `"`); err == nil {
				dm.WarningMsg = tmp
			}

			// Split presets params so we can parse them.
			attrTokens := strings.Split(items[4], " ")

			for t := 0; t < len(attrTokens); t += 2 {
				if attrTokens[t] == "-v" {
					attrs := strings.Split(attrTokens[t+1], ",")

					if len(attrs) >= 3 {
						dm.Presets[attrs[0]] = AttrConv{Conv: attrs[1], Name: attrs[2]}
					} else {
						dm.Presets[attrs[0]] = AttrConv{Conv: attrs[1]}
					}
				}
			}

			drives = append(drives, dm)
			items = make([]string, 5)
			idx = 0
		}

		prev = tok
	}

	return drives
}

func main() {
	var (
		drivedbURL, outFilename string
	)

	flag.StringVar(&drivedbURL, "url", defaultDrivedbURL, "drivedb URL")
	flag.StringVar(&outFilename, "o", "drivedb.yml", "Output .yml filename")
	flag.Parse()

	resp, err := http.Get(drivedbURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot fetch drivedb: %v\n", err)
		os.Exit(1)
	}

	defer resp.Body.Close()

	drives := parseDrivedb(resp.Body)
	fmt.Printf("Parsed drivedb.h - %d entries\n", len(drives))

	destFile, err := os.Create(outFilename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot create output: %v\n", err)
		os.Exit(1)
	}

	defer destFile.Close()

	enc := yaml.NewEncoder(destFile)

	if err := enc.Encode(DriveDb{drives}); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding yaml: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Successfully wrote output to %s\n", outFilename)
}