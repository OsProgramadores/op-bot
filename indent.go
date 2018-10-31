package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/kballard/go-shellquote"
	"html"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os/exec"
	"regexp"
	"strings"
)

type replFile struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Content string `json:"content"`
	Index   int    `json:"index"`

	indented string
}

type replProject struct {
	SessionID  string              `json:"session_id"`
	RevisionID string              `json:"revision_id"`
	EditorText string              `json:"editor_text"`
	Language   string              `json:"language"`
	Files      map[string]replFile `json:"files"`

	url                string
	newURL             string
	editorTextIndented string
	indentedByUs       bool

	IsProject bool `json:"is_project"`
}

type replaceString struct {
	src string
	dst string
}

type indenterCmd struct {
	cmd        string
	args       string
	replaceErr *replaceString
}

type indenterPrograms map[string]*indenterCmd

const (
	replBaseURL = "https://repl.it/"
	replSaveURL = "https://repl.it/save"

	// astyle uses consistent arts for C/C++/C#, and Java (we use indent for C)
	aStyleBaseArgs = "--style=java --indent=spaces=4 --indent-classes --indent-switches --indent-cases --indent-namespaces --indent-labels --indent-col1-comments --pad-oper --pad-header --quiet"
	// For python, we use yapf.
	pythonIndenterArgs = "--style='{based_on_style: pep8, indent_width: 2}'"
)

var (
	indenters = indenterPrograms{
		"c": &indenterCmd{
			cmd:        "indent",
			args:       "--no-tabs --tab-size4 --indent-level4 --braces-on-if-line --cuddle-else --braces-on-func-def-line --braces-on-struct-decl-line --cuddle-do-while --no-space-after-function-call-names --no-space-after-parentheses --dont-break-procedure-type -l666",
			replaceErr: &replaceString{src: "indent: Standard input:", dst: fmt.Sprintf("%s ", T("line"))}},
		"cpp": &indenterCmd{
			cmd:  "astyle",
			args: aStyleBaseArgs + " --mode=c"},
		"cpp11": &indenterCmd{
			cmd:  "astyle",
			args: aStyleBaseArgs + " --mode=c"},
		"csharp": &indenterCmd{
			cmd:  "astyle",
			args: aStyleBaseArgs + " --mode=cs"},
		"java": &indenterCmd{
			cmd:  "astyle",
			args: aStyleBaseArgs + " --mode=java"},
		"python": &indenterCmd{
			cmd:  "indent-python2",
			args: pythonIndenterArgs,
		},
		"python3": &indenterCmd{
			cmd:  "indent-python3",
			args: pythonIndenterArgs,
		},
		"go": &indenterCmd{
			cmd:  "indent-golang",
			args: "-s",
		},
	}
)

type execRunner interface {
	run(string, string, ...string) (string, string, error)
}

type runner struct{}

func (x runner) run(cmdIn string, command string, args ...string) (string, string, error) {
	cmd := exec.Command(command, args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return "", "", err
	}

	var outbuf, errbuf bytes.Buffer
	cmd.Stdout = &outbuf
	cmd.Stderr = &errbuf

	io.WriteString(stdin, cmdIn)
	stdin.Close()

	if err = cmd.Run(); err != nil {
		return "", errbuf.String(), err
	}

	return outbuf.String(), errbuf.String(), nil
}

func parseReplItJSON(data []byte) (*replProject, error) {
	var repl replProject
	err := json.Unmarshal(data, &repl)
	if err != nil {
		return nil, err
	}

	// Unescaping HTML.
	repl.Language = strings.ToLower(html.UnescapeString(repl.Language))
	repl.EditorText = trDelete(html.UnescapeString(repl.EditorText), "\r")
	for key, file := range repl.Files {
		file.Name = html.UnescapeString(file.Name)
		file.Content = trDelete(html.UnescapeString(file.Content), "\r")
		repl.Files[key] = file
	}

	return &repl, nil
}

func parseReplItDownload(body []byte) (*replProject, error) {
	// Repl.it now returns REPLIT_DATA in base64. Let's extract and decode it.
	regex := regexp.MustCompile(`(?s:REPLIT_DATA = JSON\.parse\(atob\('(.*?)\'\)\))`)
	match := regex.FindSubmatch(body)
	if match == nil {
		log.Println("Error extracting REPLIT_DATA base64; no matches")
		return nil, errors.New(T("error_extracting_from_replit"))
	}

	decoded, err := base64.StdEncoding.DecodeString(string(match[1]))
	if err != nil {
		log.Printf("Error decoding base64 %q: %v\n", string(match[1]), err)
		return nil, errors.New(T("error_extracting_from_replit"))
	}
	return parseReplItJSON(decoded)
}

func downloadReplIt(url string) (*replProject, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	repl, err := parseReplItDownload(body)
	if err != nil {
		return nil, err
	}

	repl.url = url
	return repl, nil

}

func uploadToRepl(repl *replProject) (*replProject, error) {
	if !repl.indentedByUs {
		return nil, errors.New(T("code_already_indented"))
	}

	formData := url.Values{"id": {repl.SessionID}, "language": {repl.Language}}

	if repl.IsProject {
		// Adding the files, if we are dealing with a project.
		i := 0
		for _, file := range repl.Files {
			formData.Add(fmt.Sprintf("files[%d][type]", i), "text/plain")
			formData.Add(fmt.Sprintf("files[%d][id]", i), fmt.Sprintf("%d", file.ID))
			formData.Add(fmt.Sprintf("files[%d][name]", i), file.Name)
			formData.Add(fmt.Sprintf("files[%d][content]", i), file.indented)
			formData.Add(fmt.Sprintf("files[%d][index]", i), fmt.Sprintf("%d", file.Index))
			i++
		}
	} else {
		// If not a project, we submit the indented editor_text.
		formData.Add("editor_text", repl.editorTextIndented)
	}

	resp, err := http.PostForm(replSaveURL, formData)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	url := repl.url
	repl, err = parseReplItJSON(body)
	if err != nil {
		fmt.Println(string(body))
		return nil, err
	}

	repl.url = url
	repl.newURL = fmt.Sprintf("%s%s/%s", replBaseURL, repl.SessionID, repl.RevisionID)

	return repl, nil
}

// nolint: gocyclo
// indentCode indents code in a repl.it snippet.
func indentCode(rr execRunner, repl *replProject, indenters indenterPrograms) (*replProject, error) {
	// Fetch correct indenter parameters
	indenter, ok := indenters[repl.Language]
	if !ok {
		return nil, fmt.Errorf(T("unknown_language"), repl.Language)
	}

	args, err := shellquote.Split(indenter.args)
	if err != nil {
		return nil, err
	}

	// Indent each of the files, if we are dealing with a project.
	if repl.IsProject {
		for key, file := range repl.Files {
			stdout, stderr, err := rr.run(file.Content, indenter.cmd, args...)
			if err != nil {
				if len(stderr) > 0 {
					errorMsg := stderr
					if indenter.replaceErr != nil {
						errorMsg = strings.Replace(errorMsg, indenter.replaceErr.src, indenter.replaceErr.dst, -1)
					}
					return nil, fmt.Errorf(T("errors_replit_file"), file.Name, errorMsg)
				}
				return nil, err
			}

			indented := stdout
			file.indented = indented
			repl.Files[key] = file

			if indented != file.Content {
				repl.indentedByUs = true
			}
		}
	} else {
		// If not a project, indent the content of EditorText.
		stdout, stderr, err := rr.run(repl.EditorText, indenter.cmd, args...)

		if err != nil {
			if len(stderr) > 0 {
				errorMsg := stderr
				if indenter.replaceErr != nil {
					errorMsg = strings.Replace(errorMsg, indenter.replaceErr.src, indenter.replaceErr.dst, -1)
				}

				return nil, fmt.Errorf(T("errors_found"), errorMsg)
			}
			return nil, err
		}

		indented := stdout
		repl.editorTextIndented = indented

		if repl.EditorText != indented {
			repl.indentedByUs = true
		}
	}
	return repl, nil
}

func handleReplItURL(rr execRunner, url string) (*replProject, error) {
	if !strings.HasPrefix(strings.ToLower(url), replBaseURL) {
		return nil, errors.New(T("invalid_replit_url"))
	}

	repl, err := downloadReplIt(url)
	if err != nil {
		return nil, errors.New(T("error_accessing_replit_url"))
	}

	repl, err = indentCode(rr, repl, indenters)
	if err != nil {
		return nil, err
	}

	repl, err = uploadToRepl(repl)
	if err != nil {
		return nil, err
	}

	return repl, nil
}
