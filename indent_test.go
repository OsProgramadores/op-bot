// Unit tests for the osprogramadores_bot.

package main

import (
	"errors"
	"fmt"
	"github.com/kylelemons/godebug/pretty"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDownloadReplIt(t *testing.T) {

	templateJSON := `<script>REPLIT_DATA = {
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
		"title":null}
		</script>
    	<script>GOVAL_TOKEN={"time_created":1499297884813,"msg_mac":"yfT1mBBT2qmOj0ahbF5EP7a3iYVm2otqSEjaf11MQiQ="}</script>
    	<script>undefined</script>
    	<script>undefined</script>
	`

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
			body: []byte(fmt.Sprintf(templateJSON, "false", "{}")),
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
			body: []byte(fmt.Sprintf(templateJSON, "true", projectJSON)),
			want: &replProject{
				SessionID:  "ABCD",
				RevisionID: "0",
				EditorText: "/* Fake program */",
				Files: map[string]replFile{
					"1545981": replFile{ID: 1545981, Name: "main.c", Content: "/*main.c*/", Index: 0},
					"1545982": replFile{ID: 1545982, Name: "file1.c", Content: "/*file1.c*/", Index: 1},
				},
				IsProject: true,
				Language:  "c",
			},
			wantError: false,
		},
		// Non replit output
		{
			body:      []byte("meh! This will break"),
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
		return "", "", errors.New("fakerunner_error")
	}

	return f.cmdOut, f.cmdErr, nil
}

func TestIndent(t *testing.T) {
	casetests := []struct {
		runner     execRunner
		repl       *replProject
		indenters  indenterPrograms
		wantRepl   *replProject
		wantRunner *fakeRunner
		wantError  bool
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
					"1234": replFile{
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
					"1234": replFile{
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
		// Error case: Missing input language.
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
		// Error case: Unsupported input language.
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
	}
}
