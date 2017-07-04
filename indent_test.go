// Unit tests for the osprogramadores_bot.

package main

import (
	"fmt"
	"github.com/kylelemons/godebug/pretty"
	"net/http"
	"net/http/httptest"
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
	`

	projectJSON := `{
		"1545981":{"id":1545981, "name":"main.c", "content":"/*main.c*/", "index":0},
		"1545982":{"id":1545982, "name":"file1.c", "content":"/*file1.c*/", "index":1}
    }`

	casetests := []struct {
		body      []byte
		want      *replProject
		WantError bool
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
			WantError: false,
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
			WantError: false,
		},
		// Non replit output
		{
			body:      []byte("meh! This will break"),
			want:      &replProject{},
			WantError: true,
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

		if !tt.WantError {
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
