package main

import (
	"bufio"
	"fmt"
	"html"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

const (
	withoutProject    = iota
	withProject       = iota
	withProblem       = iota
	withMalformedData = iota
)

type httpTest struct {
	body     string
	getJSON  string
	postJSON string
	indented []string
}

type responseRequestTest struct {
	raw  string
	resp http.Response
}

// Based on src/net/http/response_test.go.
var bodyTestResponse = []httpTest{
	// https://repl.it/JMQu -- without project.
	{
		body:     html.UnescapeString(readTestResponseBody("testdata/replit-without-project-JMQu-0")),
		getJSON:  html.UnescapeString(`{"id":10203229,"session_id":"JMQu","revision_id":"0","editor_text":"         # include    &lt;stdio.h&gt;\n         \nint  main (   ) {\n               return       0;\n}","console_dump":"gcc version 4.6.3\n&gt;&gt;&gt; ","is_project":false,"time_created":"2017-07-03T22:54:12.276Z","time_updated":null,"language":"c","files":{},"user_session":{"session_id":2192128,"user_id":187913,"title":null,"description":null,"views":0,"id":9549902},"owner":187913,"title":null}`),
		postJSON: html.UnescapeString(`{"session_id":"JMQu","revision_id":"2","files":{},"owner":187913}`),
		indented: []string{
			html.UnescapeString("#include    &lt;stdio.h&gt;\n\nint main() {\n    return 0;\n}\n")},
	},
	// https://repl.it/JK1C/1 -- with project.
	{
		body:     html.UnescapeString(readTestResponseBody("testdata/replit-with-project-JK1C-1")),
		getJSON:  html.UnescapeString(`{"id":10203109,"session_id":"JK1C","revision_id":"1","editor_text":"","console_dump":"gcc version 4.6.3\n","is_project":true,"time_created":"2017-07-03T22:46:59.363Z","time_updated":null,"language":"c","files":{"1535943":{"id":1535943,"name":"main.c","content":"int main()\n{\nreturn 0;\n}","index":0}},"user_session":{"session_id":2185068,"user_id":187913,"title":null,"description":null,"views":8,"id":9544657},"owner":187913,"title":null}`),
		postJSON: html.UnescapeString(`{"session_id":"JK1C","revision_id":"3","files":{"1536051":{"id":1536051,"name":"main.c","content":"int main() {\n    return 0;\n}\n","index":0,"prevId":1535943}},"owner":null}`),
		indented: []string{html.UnescapeString("int main() {\n    return 0;\n}\n")},
	},
}

var testGETResponse = []responseRequestTest{
	// repl.it without project.
	{
		raw: "HTTP/1.0 200 OK\r\n" +
			"Connection: close\r\n" +
			"\r\n" +
			bodyTestResponse[withoutProject].body + "\r\n",

		resp: http.Response{
			Status:     "200 OK",
			StatusCode: 200,
			Proto:      "HTTP/1.0",
			ProtoMajor: 1,
			ProtoMinor: 0,
			Request:    &http.Request{Method: "GET"},
			Header: http.Header{
				"Connection": {"close"},
			},
			Close:         true,
			ContentLength: -1,
		},
	},
	// repl.it with project.
	{
		raw: "HTTP/1.0 200 OK\r\n" +
			"Connection: close\r\n" +
			"\r\n" +
			string(bodyTestResponse[withProject].body) + "\r\n",

		resp: http.Response{
			Status:     "200 OK",
			StatusCode: 200,
			Proto:      "HTTP/1.0",
			ProtoMajor: 1,
			ProtoMinor: 0,
			Request:    &http.Request{Method: "GET"},
			Header: http.Header{
				"Connection": {"close"},
			},
			Close:         true,
			ContentLength: -1,
		},
	},
	// problem request.
	{
		raw: "HTTP/1.0 200 OK\r\n" +
			"Connection: close\r\n" +
			"\r\n" +
			"PROBLEM REQUEST\r\n",

		resp: http.Response{
			Status:     "200 OK",
			StatusCode: 200,
			Proto:      "HTTP/1.0",
			ProtoMajor: 1,
			ProtoMinor: 0,
			Request:    &http.Request{Method: "GET"},
			Header: http.Header{
				"Connection": {"close"},
			},
			Close:         true,
			ContentLength: -1,
		},
	},
	// Malformed JSON data.
	{
		raw: "HTTP/1.0 200 OK\r\n" +
			"Connection: close\r\n" +
			"\r\n" +
			`REPLIT_DATA = {foo bar}</script>` + "\r\n",

		resp: http.Response{
			Status:     "200 OK",
			StatusCode: 200,
			Proto:      "HTTP/1.0",
			ProtoMajor: 1,
			ProtoMinor: 0,
			Request:    &http.Request{Method: "GET"},
			Header: http.Header{
				"Connection": {"close"},
			},
			Close:         true,
			ContentLength: -1,
		},
	},
}

var testPOSTResponse = []responseRequestTest{
	// repl.it without project.
	{
		raw: "HTTP/1.0 200 OK\r\n" +
			"Connection: close\r\n" +
			"\r\n" +
			bodyTestResponse[withoutProject].postJSON + "\r\n",

		resp: http.Response{
			Status:     "200 OK",
			StatusCode: 200,
			Proto:      "HTTP/1.0",
			ProtoMajor: 1,
			ProtoMinor: 0,
			Request:    &http.Request{Method: "POST"},
			Header: http.Header{
				"Connection": {"close"},
			},
			Close:         true,
			ContentLength: -1,
		},
	},
	// repl.it with project.
	{
		raw: "HTTP/1.0 200 OK\r\n" +
			"Connection: close\r\n" +
			"\r\n" +
			string(bodyTestResponse[withProject].postJSON) + "\r\n",

		resp: http.Response{
			Status:     "200 OK",
			StatusCode: 200,
			Proto:      "HTTP/1.0",
			ProtoMajor: 1,
			ProtoMinor: 0,
			Request:    &http.Request{Method: "POST"},
			Header: http.Header{
				"Connection": {"close"},
			},
			Close:         true,
			ContentLength: -1,
		},
	},
}

type stubDownloader struct{}
type stubPoster struct{}

func readTestResponseBody(filename string) string {
	body, err := ioutil.ReadFile(filename)
	if err != nil {
		log.Fatalf("Problem reading test file %q: %s\n", filename, err.Error())
	}
	return html.UnescapeString(string(body))
}

func (stubDownloader) Get(url string) (*http.Response, error) {
	switch url {
	case "error":
		return nil, fmt.Errorf("Simulating an error")
	case "problem":
		return http.ReadResponse(bufio.NewReader(strings.NewReader(testGETResponse[withProblem].raw)), testGETResponse[withProblem].resp.Request)
	case "malformed":
		return http.ReadResponse(bufio.NewReader(strings.NewReader(testGETResponse[withMalformedData].raw)), testGETResponse[withMalformedData].resp.Request)
	default:
		return http.ReadResponse(bufio.NewReader(strings.NewReader(testGETResponse[withoutProject].raw)), testGETResponse[withoutProject].resp.Request)
	}
}

func (stubPoster) PostForm(url string, formData url.Values) (*http.Response, error) {
	return http.ReadResponse(bufio.NewReader(strings.NewReader(testPOSTResponse[withoutProject].raw)), testPOSTResponse[withoutProject].resp.Request)
}

func TestParseReplIt(t *testing.T) {
	// Let's send HTML instead of JSON, to break it.
	repl, err := parseReplIt([]byte(bodyTestResponse[withoutProject].body))
	if err == nil {
		t.Errorf("Expected to break, since we are giving it HTML instead of JSON")
		return
	}

	// Without project.
	repl, err = parseReplIt([]byte(bodyTestResponse[withoutProject].getJSON))
	if err != nil {
		t.Errorf(err.Error())
		return
	}

	if repl.SessionID != "JMQu" {
		t.Errorf("SessionID expected to be %q, got %q", "JMQu", repl.SessionID)
		return
	}

	if repl.RevisionID != "0" {
		t.Errorf("RevisionID expected to be %q, got %q", "0", repl.RevisionID)
		return
	}

	editorText := html.UnescapeString("         # include    &lt;stdio.h&gt;\n         \nint  main (   ) {\n               return       0;\n}")
	if repl.EditorText != editorText {
		t.Errorf("EditorText expected to be %q, got %q", editorText, repl.EditorText)
		return
	}

	if repl.IsProject {
		t.Errorf("Expected to NOT be a project")
		return
	}

	if repl.Language != "c" {
		t.Errorf("Language expected to be %q, got %q", "c", repl.Language)
		return
	}

	if len(repl.Files) > 0 {
		t.Errorf("Expected to have NO files; got %q", len(repl.Files))
		return
	}

	// With a project.
	repl, err = parseReplIt([]byte(bodyTestResponse[withProject].getJSON))
	if err != nil {
		t.Errorf(err.Error())
		return
	}

	if repl.SessionID != "JK1C" {
		t.Errorf("SessionID expected to be %q, got %q", "JK1C", repl.SessionID)
		return
	}

	if repl.RevisionID != "1" {
		t.Errorf("RevisionID expected to be %q, got %q", "1", repl.RevisionID)
		return
	}

	if repl.EditorText != "" {
		t.Errorf("EditorText expected to be empty, got %q", repl.EditorText)
		return
	}

	if !repl.IsProject {
		t.Errorf("Expected to be a project")
		return
	}

	if repl.Language != "c" {
		t.Errorf("Language expected to be %q, got %q", "c", repl.Language)
		return
	}

	if len(repl.Files) != 1 {
		t.Errorf("Expected to have %q file(s); got %q", "1", len(repl.Files))
		return
	}

	if _, ok := repl.Files["1535943"]; !ok {
		t.Errorf("Expected %q to be a key in the Files map", "1535943")
		return
	}

	file := repl.Files["1535943"]

	if file.ID != 1535943 {
		t.Errorf("Expected file ID to be %q, got %q", "1535943", file.ID)
		return
	}

	if file.Name != "main.c" {
		t.Errorf("Expected file name to be %q, got %q", "main.c", file.Name)
		return
	}

	content := "int main()\n{\nreturn 0;\n}"
	if file.Content != content {
		t.Errorf("Expected file content to be %q, got %q", content, file.Content)
		return
	}

	if file.Index != 0 {
		t.Errorf("Expected file index to be %q, got %q", "0", file.Index)
		return
	}
}

func TestDownload(t *testing.T) {
	stub := stubDownloader{}

	// This should simulate an error
	url := "error"
	repl, err := downloadReplIt(stub, url)
	if err == nil {
		t.Errorf("Expected to break, since we are simulating an error")
		return
	}

	// This should simulate a problematic request.
	url = "problem"
	repl, err = downloadReplIt(stub, url)
	if err == nil {
		t.Errorf("Expected to break, since we are simulating a problematic request")
		return
	}

	// This should simulate a request with malformed JSON.
	url = "malformed"
	repl, err = downloadReplIt(stub, url)
	if err == nil {
		t.Errorf("Expected to break, since we are simulating a malformed JSON")
		return
	}

	url = "foo bar"
	repl, err = downloadReplIt(stub, url)
	if err != nil {
		t.Errorf(err.Error())
		return
	}

	testRepl, err := parseReplIt([]byte(bodyTestResponse[withoutProject].getJSON))
	if err != nil {
		t.Errorf(err.Error())
		return
	}
	testRepl.url = url

	if !reflect.DeepEqual(repl, testRepl) {
		t.Errorf("repl projects expected to be the same")
		return
	}
}

func TestIndentCode(t *testing.T) {
	testRepl, err := parseReplIt([]byte(bodyTestResponse[withoutProject].getJSON))
	if err != nil {
		t.Errorf(err.Error())
		return
	}

	repl, err := indentCode(exec.Command, testRepl, indenters[testRepl.Language])
	if !repl.indentedByUs {
		t.Errorf("Expected to have been indented by us")
		return
	}

	if repl.editorTextIndented != bodyTestResponse[withoutProject].indented[0] {
		t.Errorf("Incorrect indentation: %s", repl.editorTextIndented)
		return
	}

	// Let's now try to indent something already indented.
	repl.EditorText = repl.editorTextIndented
	repl, err = indentCode(exec.Command, repl, indenters[repl.Language])
	if err != nil {
		t.Errorf(err.Error())
		return
	}

	if repl.indentedByUs {
		t.Errorf("Expected to be already indented")
		return
	}

	// And now let's try to indent something with a syntax error.
	repl.EditorText = "#inc lude\n {"
	repl, err = indentCode(exec.Command, repl, indenters[repl.Language])
	if err == nil {
		t.Errorf("Expected to give an error due to the syntax problem")
		return
	}

	// Now let's indent a project.
	testRepl, err = parseReplIt([]byte(bodyTestResponse[withProject].getJSON))
	if err != nil {
		t.Errorf(err.Error())
		return
	}

	repl, err = indentCode(exec.Command, testRepl, indenters[testRepl.Language])
	if !repl.indentedByUs {
		t.Errorf("Expected to have been indented by us")
		return
	}

	i := 0
	for _, file := range repl.Files {
		if file.indented != bodyTestResponse[withProject].indented[i] {
			t.Errorf("Incorrect indentation for file %q: %s", file.Name, file.indented)
			return
		}
	}

}

func TestIndent(t *testing.T) {
	testRepl, err := parseReplIt([]byte(bodyTestResponse[withoutProject].getJSON))
	if err != nil {
		t.Errorf(err.Error())
		return
	}

	repl, err := indent(testRepl)
	if err != nil {
		t.Errorf(err.Error())
		return
	}

	if repl.editorTextIndented != bodyTestResponse[withoutProject].indented[0] {
		t.Errorf("Incorrect indentation: %s", repl.editorTextIndented)
		return
	}
}

func TestUploadReplit(t *testing.T) {
	stub := stubPoster{}

	// Without project.
	testRepl, err := parseReplIt([]byte(bodyTestResponse[withoutProject].getJSON))
	if err != nil {
		t.Errorf(err.Error())
		return
	}

	testRepl, err = indent(testRepl)

	if err != nil {
		t.Errorf(err.Error())
		return
	}

	_, err = uploadReplIt(stub, testRepl)
	if err != nil {
		t.Errorf(err.Error())
		return
	}

	// Now let's upload a project.
	testRepl, err = parseReplIt([]byte(bodyTestResponse[withProject].getJSON))
	if err != nil {
		t.Errorf(err.Error())
		return
	}

	testRepl, err = indent(testRepl)

	if err != nil {
		t.Errorf(err.Error())
		return
	}

	_, err = uploadReplIt(stub, testRepl)
	if err != nil {
		t.Errorf(err.Error())
		return
	}
}
