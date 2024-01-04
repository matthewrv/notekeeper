package notekeeper

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSaveNote(t *testing.T) {
	note := "Sample note"

	// save note successfully
	filename, err := SaveNote(note)
	require.Equal(t, nil, err, "Function should have finished without errors")
	require.FileExists(t, storage_dir+filename, "File was not actually saved")
	err = os.Remove(storage_dir + filename)
	require.Truef(t, err == nil, "fucked up teardown: %s", err)
}

func TestLoadNote(t *testing.T) {
	awaited := "Sample note"

	// Test simple note
	content, err := LoadNote("test.md")
	require.Equal(t, nil, err, "Function should have finished wihtout errors")
	require.Equal(t, awaited, content, "Content of file does not match expected result")

	// Test note with runes
	awaited = "Тестовая заметка ✨"
	content, err = LoadNote("test_runes.md")
	require.Equal(t, nil, err, "Function should have finished wihtout errors")
	require.Equal(t, awaited, content, "Content of file does not match expected result")

	// Test non existing note
	content, err = LoadNote("non_existing_note.md")
	require.NotNil(t, err, "Function should have failed for non existing note")
	require.Equal(t, "", content, "Content sould be empty if note loading failed")
}

func TestNameSanitizer(t *testing.T) {
	var cases = []struct {
		in, expect string
	}{
		{"Test note", "test-note"},
		{"Тестовая заметка ✨", "тестовая-заметка"}, // alphanumeric symbols only
		{"Test note|[]:?*\\\"", "test-note"},       // forbidden file symbols
	}

	for _, c := range cases {
		got := sanitizeName(c.in)
		assert.Equal(t, c.expect, got, "Sanitized name does not match")
	}
}
