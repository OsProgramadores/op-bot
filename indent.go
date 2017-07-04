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

type httpGetter interface {
	Get(string) (*http.Response, error)
}

type httpPoster interface {
	PostForm(string, url.Values) (*http.Response, error)
}

func parseReplIt(data []byte) (*replProject, error) {
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

func downloadReplIt(h httpGetter, url string) (*replProject, error) {
	resp, err := h.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)

	regex, err := regexp.Compile("REPLIT_DATA = ({.*})</script>")
	if err != nil {
		return nil, err
	}

	match := regex.FindSubmatch(body)
	if match == nil {
		return nil, fmt.Errorf("não foi possível extrair os dados do repl.it: %s", string(body))
	}

	repl, err := parseReplIt(match[1])
	if err != nil {
		return nil, err
	}

	repl.url = url
	return repl, nil

}

func uploadReplIt(poster httpPoster, repl *replProject) (*replProject, error) {
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

	resp, err := poster.PostForm(replSaveURL, formData)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)

	url := repl.url
	repl, err = parseReplIt(body)
	if err != nil {
		fmt.Println(string(body))
		return nil, err
	}

	repl.url = url
	repl.newURL = fmt.Sprintf("%s%s/%s", replBaseURL, repl.SessionID, repl.RevisionID)

	return repl, nil
}

func indentCode(command func(string, ...string) *exec.Cmd, repl *replProject, indenter *indenterCmd) (*replProject, error) {
	// Let's reset this flag.
	repl.indentedByUs = false

	// Indent each of the files, if we are dealing with a project.
	if repl.IsProject {
		for key, file := range repl.Files {
			cmd := command(indenter.cmd, strings.Split(indenter.args, " ")...)
			stdin, err := cmd.StdinPipe()
			if err != nil {
				return nil, err
			}

			var outbuf, errbuf bytes.Buffer
			cmd.Stdout = &outbuf
			cmd.Stderr = &errbuf

			io.WriteString(stdin, file.Content)
			stdin.Close()

			if err = cmd.Run(); err != nil {
				if len(errbuf.String()) > 0 {
					errorMsg := errbuf.String()
					if indenter.replaceErr != nil {
						errorMsg = strings.Replace(errorMsg, indenter.replaceErr.src, indenter.replaceErr.dst, -1)
					}
					return nil, fmt.Errorf("erros detectados no arquivo %s:\n%s", file.Name, errorMsg)
				}
				return nil, err
			}

			indented := outbuf.String()
			file.indented = indented
			repl.Files[key] = file

			if indented != file.Content {
				repl.indentedByUs = true
			}
		}
	} else {
		// If not a project, indent the content of EditorText.
		cmd := command(indenter.cmd, strings.Split(indenter.args, " ")...)
		stdin, err := cmd.StdinPipe()
		if err != nil {
			return nil, err
		}

		var outbuf, errbuf bytes.Buffer
		cmd.Stdout = &outbuf
		cmd.Stderr = &errbuf

		io.WriteString(stdin, repl.EditorText)
		stdin.Close()

		if err = cmd.Run(); err != nil {
			if len(errbuf.String()) > 0 {
				errorMsg := errbuf.String()
				if indenter.replaceErr != nil {
					errorMsg = strings.Replace(errorMsg, indenter.replaceErr.src, indenter.replaceErr.dst, -1)
				}

				return nil, fmt.Errorf("erros detectados:\n%s", errorMsg)
			}
			return nil, err
		}

		indented := outbuf.String()
		repl.editorTextIndented = indented

		if repl.EditorText != indented {
			repl.indentedByUs = true
		}
	}
	return repl, nil
}

func indent(repl *replProject) (*replProject, error) {
	switch repl.Language {
	case "c":
		return indentCode(exec.Command, repl, indenters[repl.Language])
	default:
		return nil, fmt.Errorf("ainda não sei indentar essa linguagem %q. Se puder ajudar, faça um pull request para https://github.com/OsProgramadores/osprogramadores_bot :)", repl.Language)
	}
}

func handleReplItURL(url string) (*replProject, error) {
	if !strings.HasPrefix(strings.ToLower(url), replBaseURL) {
		return nil, fmt.Errorf("esta não é uma URL válida do repl.it")
	}

	httpClient := &http.Client{}

	repl, err := downloadReplIt(httpClient, url)
	if err != nil {
		return nil, fmt.Errorf("não foi possível acessar esta URL do repl.it")
	}

	repl, err = indent(repl)
	if err != nil {
		return nil, err
	}

	repl, err = uploadReplIt(httpClient, repl)
	if err != nil {
		return nil, err
	}

	return repl, nil
}
