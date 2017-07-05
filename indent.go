package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"io/ioutil"
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
	IsProject  bool                `json:"is_project"`
	Language   string              `json:"language"`
	Files      map[string]replFile `json:"files"`

	url                string
	newURL             string
	editorTextIndented string
	indentedByUs       bool
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

const (
	replBaseURL = "https://repl.it/"
	replSaveURL = "https://repl.it/save"
)

var (
	indenters = map[string]*indenterCmd{
		"c": &indenterCmd{cmd: "indent", args: "--no-tabs --tab-size4 --indent-level4 --braces-on-if-line --cuddle-else --braces-on-func-def-line --braces-on-struct-decl-line --cuddle-do-while --no-space-after-function-call-names --no-space-after-parentheses --dont-break-procedure-type -l666", replaceErr: &replaceString{src: "indent: Standard input:", dst: "linha "}},
	}
)

type execRunner interface {
	run(string, string, ...string) (string, string, error)
}

type runner struct{}

func (x runner) run(command string, cmdIn string, args ...string) (string, string, error) {
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
		return "", "", err
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
	repl.EditorText = html.UnescapeString(repl.EditorText)
	for key, file := range repl.Files {
		file.Name = html.UnescapeString(file.Name)
		file.Content = html.UnescapeString(file.Content)
		repl.Files[key] = file
	}

	return &repl, nil
}

func parseReplItDownload(body []byte) (*replProject, error) {
	regex := regexp.MustCompile("(?s:REPLIT_DATA = ({.*}).*</script>)")
	match := regex.FindSubmatch(body)
	if match == nil {
		return nil, fmt.Errorf("não foi possível extrair os dados do repl.it")
	}
	return parseReplItJSON(match[1])
}

func downloadReplIt(url string) (*replProject, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)

	repl, err := parseReplItDownload(body)
	if err != nil {
		return nil, err
	}

	repl.url = url
	return repl, nil

}

func uploadToRepl(repl *replProject) (*replProject, error) {
	if !repl.indentedByUs {
		return nil, fmt.Errorf("código já estava indentado :)")
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

func indentCode(rr execRunner, repl *replProject, indenter *indenterCmd) (*replProject, error) {
	// Indent each of the files, if we are dealing with a project.
	if repl.IsProject {
		for key, file := range repl.Files {
			stdout, stderr, err := rr.run(file.Content, indenter.cmd, strings.Split(indenter.args, " ")...)
			if err != nil {
				if len(stderr) > 0 {
					errorMsg := stderr
					if indenter.replaceErr != nil {
						errorMsg = strings.Replace(errorMsg, indenter.replaceErr.src, indenter.replaceErr.dst, -1)
					}
					return nil, fmt.Errorf("erros detectados no arquivo %s:\n%s", file.Name, errorMsg)
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
		stdout, stderr, err := rr.run(repl.EditorText, indenter.cmd, strings.Split(indenter.args, " ")...)
		if err != nil {
			if len(stderr) > 0 {
				errorMsg := stderr
				if indenter.replaceErr != nil {
					errorMsg = strings.Replace(errorMsg, indenter.replaceErr.src, indenter.replaceErr.dst, -1)
				}

				return nil, fmt.Errorf("erros detectados:\n%s", errorMsg)
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

func indent(rr execRunner, repl *replProject) (*replProject, error) {
	switch repl.Language {
	case "c":
		return indentCode(rr, repl, indenters[repl.Language])
	default:
		return nil, fmt.Errorf("ainda não sei indentar essa linguagem %q. Se puder ajudar, faça um pull request para https://github.com/OsProgramadores/osprogramadores_bot :)", repl.Language)
	}
}

func handleReplItURL(rr execRunner, url string) (*replProject, error) {
	if !strings.HasPrefix(strings.ToLower(url), replBaseURL) {
		return nil, fmt.Errorf("esta não é uma URL válida do repl.it")
	}

	repl, err := downloadReplIt(url)
	if err != nil {
		return nil, fmt.Errorf("não foi possível acessar esta URL do repl.it")
	}

	repl, err = indent(rr, repl)
	if err != nil {
		return nil, err
	}

	repl, err = uploadToRepl(repl)
	if err != nil {
		return nil, err
	}

	return repl, nil
}
