package notekeeper

import (
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"time"
)

const storage_dir = "./notes/"

var forbidden = regexp.MustCompile("[:*?\"></\\| ]")
var nonAlphaNumeric = regexp.MustCompile(`[^\p{L}\p{N} ]+`)

func sanitizeName(name string) string {
	name = strings.ToLower(name)
	name = nonAlphaNumeric.ReplaceAllLiteralString(name, "")
	name = strings.Trim(name, " \t")
	name = forbidden.ReplaceAllString(name, "-")
	return name
}

func prepareNoteName(heading string) string {
	heading = sanitizeName(heading)
	bound := min(len(heading), 60) // take only 60 first characters for name
	heading = heading[:bound]

	id := time.Now().Format("2006-01-02T15-04-05")
	return fmt.Sprintf("%s_%s.md", id, heading)
}

func SaveNote(note string) (string, error) {
	err := os.MkdirAll(storage_dir, 0666)
	if err != nil {
		log.Printf("Error creating directory to save note: %s", err)
		return "", err
	}

	heading := strings.Split(note, "\n")[0]
	filename := prepareNoteName(heading)

	err = os.WriteFile(storage_dir+filename, []byte(note), 0666)
	if err != nil {
		log.Printf("Error occured saving not to file: %s", err)
		return "", err
	}
	return filename, nil
}

func LoadNote(name string) (string, error) {
	content, err := os.ReadFile(storage_dir + name)
	if err != nil {
		log.Printf("Failed reading note %s: %s", name, err)
		return "", err
	}

	return string(content), nil
}
