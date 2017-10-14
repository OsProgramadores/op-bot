// Unit tests for op-bot.

package main

import (
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/kylelemons/godebug/pretty"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func prepareDownloadBody(bodyTemplate, jsonData string) []byte {
	return []byte(fmt.Sprintf(bodyTemplate, base64.StdEncoding.EncodeToString([]byte(jsonData))))
}

func TestDownloadReplIt(t *testing.T) {
	templateDownload := `<script>REPLIT_DATA = JSON.parse(atob('%s'))</script>`
	templateJSON := `{
		"id":10126000,
		"session_id":"ABCD",
		"revision_id":"0",
		"editor_text":"/* Fake program */",
		"console_dump":"gcc version 4.6.3\n",
		"is_project":%s,
		"time_created":"2017-06-30T06:22:17.570Z",
		"time_updated":null,
		"language":"c",
		"files":%s,
		"user_session":{
		  "session_id":77521,
		  "user_id":9920,
		  "title":null,
		  "description":null,
		  "views":12,
		  "id":38889},
		"owner":9920,
		"title":null
	}`

	projectJSON := `{
		"1545981":{"id":1545981, "name":"main.c", "content":"/*main.c*/", "index":0},
		"1545982":{"id":1545982, "name":"file1.c", "content":"/*file1.c*/", "index":1}
    }`

	casetests := []struct {
		body      []byte
		want      *replProject
		wantError bool
	}{
		// Valid JSON, single file
		{
			body: prepareDownloadBody(templateDownload, fmt.Sprintf(templateJSON, "false", "{}")),
			want: &replProject{
				SessionID:  "ABCD",
				RevisionID: "0",
				EditorText: "/* Fake program */",
				IsProject:  false,
				Language:   "c",
			},
			wantError: false,
		},
		// Valid JSON, multi-file project
		{
			body: prepareDownloadBody(templateDownload, fmt.Sprintf(templateJSON, "true", projectJSON)),
			want: &replProject{
				SessionID:  "ABCD",
				RevisionID: "0",
				EditorText: "/* Fake program */",
				Files: map[string]replFile{
					"1545981": {ID: 1545981, Name: "main.c", Content: "/*main.c*/", Index: 0},
					"1545982": {ID: 1545982, Name: "file1.c", Content: "/*file1.c*/", Index: 1},
				},
				IsProject: true,
				Language:  "c",
			},
			wantError: false,
		},
		// Non replit output.
		{
			body:      []byte("meh! This will break"),
			want:      &replProject{},
			wantError: true,
		},
		// replit output, invalid JSON inside.
		{
			body:      prepareDownloadBody(templateDownload, "{b0rken}"),
			want:      &replProject{},
			wantError: true,
		},
	}

	for _, tt := range casetests {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w, "%s", tt.body)
		}))
		defer ts.Close()

		got, err := downloadReplIt(ts.URL)

		// Save httptest generated URL into our expected output.
		tt.want.url = ts.URL

		if !tt.wantError {
			if err != nil {
				t.Fatalf("Got error %q want no error", err)
			}

			if diff := pretty.Compare(got, tt.want); diff != "" {
				t.Errorf("diff: (-got +want)\n%s", diff)
			}
			continue
		}

		// Here, we want to see an error.
		if err == nil {
			t.Errorf("Got no error, want error")
		}
	}
}

type fakeRunner struct {
	inputCommand string
	inputArgs    string
	inputContent string
	cmdOut       string
	cmdErr       string
	returnErr    bool
}

// run simulates running an external binary.
func (f *fakeRunner) run(cmdIn string, command string, args ...string) (string, string, error) {
	f.inputCommand = command
	f.inputArgs = strings.Join(args, " ")
	f.inputContent = cmdIn

	if f.returnErr {
		return "", f.cmdErr, errors.New("fakerunner_error")
	}

	return f.cmdOut, f.cmdErr, nil
}

func TestIndent(t *testing.T) {
	casetests := []struct {
		runner       execRunner
		repl         *replProject
		indenters    indenterPrograms
		wantRepl     *replProject
		wantRunner   *fakeRunner
		wantError    bool
		wantErrorMsg string
	}{
		// Base Indent case (single file).
		{
			runner: &fakeRunner{
				cmdOut: "fake_indented_stdout",
			},
			repl: &replProject{
				IsProject:  false,
				Language:   "c",
				EditorText: "fake_editor_text",
			},
			indenters: indenterPrograms{
				"c": &indenterCmd{
					cmd:  "fake_command",
					args: "--aaa --bbb --ccc",
				},
			},
			wantRepl: &replProject{
				editorTextIndented: "fake_indented_stdout",
				EditorText:         "fake_editor_text",
				Language:           "c",
				indentedByUs:       true,
			},
			wantRunner: &fakeRunner{
				inputCommand: "fake_command",
				inputArgs:    "--aaa --bbb --ccc",
				inputContent: "fake_editor_text",
				cmdOut:       "fake_indented_stdout",
			},
		},
		// Multi-file project, one file for simplicity.
		// TODO: Add support for multiple files, eventually.
		{
			runner: &fakeRunner{
				cmdOut: "fake_indented_stdout",
			},
			repl: &replProject{
				IsProject: true,
				Files: map[string]replFile{
					"1234": {
						ID:      1234,
						Name:    "foo.c",
						Content: "fake_editor_text",
						Index:   1,
					},
				},
				Language: "c",
			},
			indenters: indenterPrograms{
				"c": &indenterCmd{
					cmd:  "fake_command",
					args: "--aaa --bbb --ccc",
				},
			},
			wantRepl: &replProject{
				IsProject: true,
				Files: map[string]replFile{
					"1234": {
						ID:       1234,
						Name:     "foo.c",
						Content:  "fake_editor_text",
						indented: "fake_indented_stdout",
						Index:    1,
					},
				},
				Language:     "c",
				indentedByUs: true,
			},
			wantRunner: &fakeRunner{
				inputCommand: "fake_command",
				inputArgs:    "--aaa --bbb --ccc",
				inputContent: "fake_editor_text",
				cmdOut:       "fake_indented_stdout",
			},
		},
		// Single file Error case: Missing input language.
		{
			runner: &fakeRunner{
				cmdOut: "fake_indented_stdout",
			},
			repl: &replProject{},
			indenters: indenterPrograms{
				"c": &indenterCmd{},
			},
			wantRepl:   &replProject{},
			wantRunner: &fakeRunner{},
			wantError:  true,
		},
		// Single File Error case: Unsupported input language.
		{
			runner: &fakeRunner{
				cmdOut: "fake_indented_stdout",
			},
			repl: &replProject{
				Language: "invalid-language",
			},
			indenters: indenterPrograms{
				"c": &indenterCmd{},
			},
			wantRepl:   &replProject{},
			wantRunner: &fakeRunner{},
			wantError:  true,
		},
		// Single file Error case: error, no stderr
		{
			runner: &fakeRunner{
				cmdOut:    "fake_indented_stdout",
				returnErr: true,
			},
			repl: &replProject{
				Language: "c",
			},
			indenters: indenterPrograms{
				"c": &indenterCmd{},
			},
			wantRepl:   &replProject{},
			wantRunner: &fakeRunner{},
			wantError:  true,
		},
		// Single File Error case: stderr message, no replaceErr.
		{
			runner: &fakeRunner{
				cmdOut:    "fake_indented_stdout",
				cmdErr:    "fake_stderr",
				returnErr: true,
			},
			repl: &replProject{
				Language: "c",
			},
			indenters: indenterPrograms{
				"c": &indenterCmd{},
			},
			wantRepl:     &replProject{},
			wantRunner:   &fakeRunner{},
			wantError:    true,
			wantErrorMsg: "fake_stderr",
		},
		// Single file Error case: stderr message, replaceErr specified.
		{
			runner: &fakeRunner{
				cmdOut:    "fake_indented_stdout",
				cmdErr:    "fake_stderr",
				returnErr: true,
			},
			repl: &replProject{
				Language: "c",
			},
			indenters: indenterPrograms{
				"c": &indenterCmd{
					replaceErr: &replaceString{
						src: "fake",
						dst: "foobar",
					},
				},
			},
			wantRepl:     &replProject{},
			wantRunner:   &fakeRunner{},
			wantError:    true,
			wantErrorMsg: "foobar_stderr",
		},
		// Multi file Error case: Missing input language.
		{
			runner: &fakeRunner{
				cmdOut: "fake_indented_stdout",
			},
			repl: &replProject{
				IsProject: true,
				Files: map[string]replFile{
					"1234": {
						ID:      1234,
						Name:    "foo.c",
						Content: "fake_editor_text",
						Index:   1,
					},
				},
			},
			indenters: indenterPrograms{
				"c": &indenterCmd{},
			},
			wantRepl:   &replProject{},
			wantRunner: &fakeRunner{},
			wantError:  true,
		},
		// Multi File Error case: Unsupported input language.
		{
			runner: &fakeRunner{
				cmdOut: "fake_indented_stdout",
			},
			repl: &replProject{
				IsProject: true,
				Files: map[string]replFile{
					"1234": {
						ID:      1234,
						Name:    "foo.c",
						Content: "fake_editor_text",
						Index:   1,
					},
				},
				Language: "invalid-language",
			},
			indenters: indenterPrograms{
				"c": &indenterCmd{},
			},
			wantRepl:   &replProject{},
			wantRunner: &fakeRunner{},
			wantError:  true,
		},
		// Multi file Error case: error, no stderr
		{
			runner: &fakeRunner{
				cmdOut:    "fake_indented_stdout",
				returnErr: true,
			},
			repl: &replProject{
				IsProject: true,
				Files: map[string]replFile{
					"1234": {
						ID:      1234,
						Name:    "foo.c",
						Content: "fake_editor_text",
						Index:   1,
					},
				},
				Language: "c",
			},
			indenters: indenterPrograms{
				"c": &indenterCmd{},
			},
			wantRepl:   &replProject{},
			wantRunner: &fakeRunner{},
			wantError:  true,
		},
		// Multi File Error case: stderr message, no replaceErr.
		{
			runner: &fakeRunner{
				cmdOut:    "fake_indented_stdout",
				cmdErr:    "fake_stderr",
				returnErr: true,
			},
			repl: &replProject{
				IsProject: true,
				Files: map[string]replFile{
					"1234": {
						ID:      1234,
						Name:    "foo.c",
						Content: "fake_editor_text",
						Index:   1,
					},
				},
				Language: "c",
			},
			indenters: indenterPrograms{
				"c": &indenterCmd{},
			},
			wantRepl:     &replProject{},
			wantRunner:   &fakeRunner{},
			wantError:    true,
			wantErrorMsg: "fake_stderr",
		},
		// Multi file Error case: stderr message, replaceErr specified.
		{
			runner: &fakeRunner{
				cmdOut:    "fake_indented_stdout",
				cmdErr:    "fake_stderr",
				returnErr: true,
			},
			repl: &replProject{
				IsProject: true,
				Files: map[string]replFile{
					"1234": {
						ID:      1234,
						Name:    "foo.c",
						Content: "fake_editor_text",
						Index:   1,
					},
				},
				Language: "c",
			},
			indenters: indenterPrograms{
				"c": &indenterCmd{
					replaceErr: &replaceString{
						src: "fake",
						dst: "foobar",
					},
				},
			},
			wantRepl:     &replProject{},
			wantRunner:   &fakeRunner{},
			wantError:    true,
			wantErrorMsg: "foobar_stderr",
		},
	}

	for _, tt := range casetests {
		gotRepl, err := indentCode(tt.runner, tt.repl, tt.indenters)

		if !tt.wantError {
			if err != nil {
				t.Fatalf("Got error %q want no error", err)
			}

			if diff := pretty.Compare(gotRepl, tt.wantRepl); diff != "" {
				t.Fatalf("diff Repl: (-got +want)\n%s", diff)
			}
			// tt.runner is modified diretly by the fake run function.
			if diff := pretty.Compare(tt.runner, tt.wantRunner); diff != "" {
				t.Fatalf("diff runner: (-got +want)\n%s", diff)
			}
			continue
		}

		// Here, we want to see an error.
		if err == nil {
			t.Fatalf("Got no error, want error")
		}

		// At this point, we want an error and we have one
		if tt.wantErrorMsg != "" {
			if !strings.Contains(err.Error(), tt.wantErrorMsg) {
				t.Fatalf("Error message mismatch. got %q want to see %q in message.", err, tt.wantErrorMsg)
			}
		}
	}
}
