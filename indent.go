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

const (
	replBaseURL = "https://repl.it"
	replSaveURL = "https://repl.it/save"
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

func parseReplItDownload(body *[]byte) (*replProject, error) {
	regex := regexp.MustCompile("REPLIT_DATA = ({.*})</script>")
	match := regex.FindSubmatch(*body)
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

	repl, err := parseReplItDownload(&body)
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
	repl.newURL = fmt.Sprintf("%s/%s/%s", replBaseURL, repl.SessionID, repl.RevisionID)

	return repl, nil
}

func indentC(repl *replProject) (*replProject, error) {
	if repl.Language != "c" {
		return nil, fmt.Errorf("essa linguagem não é C")
	}

	// Magic arguments by @mpaganini: https://github.com/marcopaganini/sock
	args := "--no-tabs --tab-size4 --indent-level4 --braces-on-if-line --cuddle-else --braces-on-func-def-line --braces-on-struct-decl-line --cuddle-do-while --no-space-after-function-call-names --no-space-after-parentheses --dont-break-procedure-type -l666"

	// Indent each of the files, if we are dealing with a project.
	if repl.IsProject {
		for key, file := range repl.Files {
			cmd := exec.Command("indent", strings.Split(args, " ")...)
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
					return nil, fmt.Errorf("erros detectados no arquivo %s:\n%s", file.Name, strings.Replace(errbuf.String(), "indent: Standard input:", "linha ", -1))
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
		cmd := exec.Command("indent", strings.Split(args, " ")...)
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
				return nil, fmt.Errorf("erros detectados:\n%s", strings.Replace(errbuf.String(), "indent: Standard input:", "linha ", -1))
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
		return indentC(repl)
	default:
		return nil, fmt.Errorf("ainda não sei indentar essa linguagem. Se puder ajudar, faça um pull request para https://github.com/OsProgramadores/osprogramadores_bot :)")
	}
}
