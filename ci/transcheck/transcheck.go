// Check that all translation IDs references in source files are defined in the
// translation files (and vice-versa).

package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"
	"regexp"

	"github.com/BurntSushi/toml"
)

func main() {
	var (
		srcDir   = flag.String("source-dir", ".", "Directory for Go sources.")
		transDir = flag.String("translations-dir", ".", "Directory for Translation files.")
	)

	flag.Parse()

	srcFiles, err := globFiles(*srcDir, "*.go")
	if err != nil {
		log.Fatalf("Error reading source files: %v", err)
	}
	referenced, err := tmessages(srcFiles)
	if err != nil {
		log.Fatalf("Error reading Tfunc calls from source files: %v", err)
	}

	transFiles, err := globFiles(*transDir, "*.toml")
	if err != nil {
		log.Fatalf("Error reading translation files: %v", err)
	}
	for _, tfile := range transFiles {
		defined, err := translationID(tfile)
		if err != nil {
			log.Fatalf("Error reading translation IDs from template files: %v", err)
		}

		// If differences exist, print them and exit with an error code.
		rDiff, dDiff := mapDiff(referenced, defined)
		if len(rDiff) != 0 || len(dDiff) != 0 {
			for k := range dDiff {
				log.Printf("%s: ID %q defined, but not referenced.\n", filepath.Base(tfile), k)
			}
			for k := range rDiff {
				log.Printf("ID %q referenced in the source files, but not defined.\n", k)
			}
			os.Exit(1)
		}
	}
}

// tmessages returns a map[string]interface{} with all Tfunc call arguments in every
// file name in the slice of filenames, or nil if no calls can be found.
func tmessages(fnames []string) (map[string]interface{}, error) {
	ret := map[string]interface{}{}

	// We match T("string"). Anything else voids the warranty.
	re, err := regexp.Compile(`\bT\("([^"]*)"\)`)
	if err != nil {
		return nil, err
	}

	for _, fname := range fnames {
		buf, err := os.ReadFile(fname)
		if err != nil {
			return nil, err
		}
		matches := re.FindAllSubmatch(buf, -1)
		for _, m := range matches {
			ret[string(m[1])] = true
		}
	}
	return ret, nil
}

// translationIDs returns all the translation IDs defined in the
// passed filename. It expects the file to be in toml format.
func translationID(tfile string) (map[string]interface{}, error) {
	var translation map[string]interface{}

	buf, err := os.ReadFile(tfile)
	if err != nil {
		return nil, err
	}
	if _, err := toml.Decode(string(buf), &translation); err != nil {
		return nil, err
	}
	return translation, nil
}

// mapDiff returns two map[string]bool maps with the matching keys from
// the input maps removed.
func mapDiff(a, b map[string]interface{}) (map[string]bool, map[string]bool) {
	reta := map[string]bool{}
	retb := map[string]bool{}

	// Initialize the return maps
	for k := range a {
		reta[k] = true
	}
	for k := range b {
		retb[k] = true
	}

	// Remove all matches
	for k := range reta {
		if _, ok := retb[k]; ok {
			delete(reta, k)
			delete(retb, k)
		}
	}
	return reta, retb
}

// globFiles returns all files under the source directory matching the specified glob.
func globFiles(src, pat string) ([]string, error) {
	matches, err := filepath.Glob(filepath.Join(src, pat))
	if err != nil {
		return nil, err
	}

	// Only consider regular files.
	ret := []string{}
	for _, f := range matches {
		fi, err := os.Stat(f)
		if err != nil {
			return nil, err
		}
		if fi.Mode().IsRegular() {
			ret = append(ret, f)
		}
	}
	return ret, nil
}
