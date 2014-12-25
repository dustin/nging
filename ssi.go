package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"path/filepath"
	"regexp"
)

var (
	ssipat  = regexp.MustCompile(`(<!--#include virtual=".*"-->)`)
	pathpat = regexp.MustCompile(`<!--#include virtual="(.*)"-->`)
)

func processSSI(root, path string) (rv []byte, err error) {
	src, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	out := ssipat.ReplaceAllFunc(src,
		func(something []byte) []byte {
			pathParts := pathpat.FindSubmatch(something)
			if len(pathParts) != 2 {
				log.Panicf("Expected to match the path, found %s",
					pathParts)
			}
			ipath := fmt.Sprintf("%s", pathParts[1])
			dir := root
			if !filepath.IsAbs(ipath) {
				dir = filepath.Dir(path)
			}
			ipath = filepath.Join(dir, ipath)
			incData, err := ioutil.ReadFile(ipath)
			if err != nil {
				return []byte(fmt.Sprintf("Error including %s: %v",
					ipath, err))
			}
			return incData
		})
	return out, nil
}
