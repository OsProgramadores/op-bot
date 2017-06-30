package main

import (
	"bytes"
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

type replData struct {
	id       string
	language string
	url      string
	newURL   string
	code     string
	indented string
	errors   string
}

func extractReplItTag(body *[]byte, tag string) (string, error) {
	regex := regexp.MustCompile(fmt.Sprintf("%q:%q", tag, "(.*?)"))
	match := regex.FindSubmatch(*body)
	if match == nil {
		return "", fmt.Errorf("não foi possível extrair tag %s", tag)
	}
	return string(match[1]), nil
}

func downloadReplIt(url string) (*replData, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)

	repl := replData{url: url}

	var start, end int
	// Code.
	if start = bytes.Index(body, []byte("<pre>")); start != -1 {
		if end = bytes.Index(body, []byte("</pre>")); end != -1 {
			repl.code = html.UnescapeString(string(body[start+len("<pre>") : end]))
		}
	}

	// Session ID.
	id, err := extractReplItTag(&body, "session_id")
	if err != nil {
		return nil, err
	}

	repl.id = id
	// Language.
	lang, err := extractReplItTag(&body, "language")
	if err != nil {
		return nil, err
	}
	repl.language = lang

	return &repl, nil

}

func uploadIndentedCode(repl *replData) (*replData, error) {
	if len(repl.indented) == 0 {
		return nil, fmt.Errorf("código não foi indentado")
	}

	if repl.code == repl.indented {
		repl.newURL = repl.url
		return nil, fmt.Errorf("código já estava indentado :)")
	}

	formData := url.Values{"id": {repl.id}, "language": {repl.language}, "editor_text": {repl.indented}}
	resp, err := http.PostForm(replSaveURL, formData)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)

	fmt.Println(string(body))

	id, err := extractReplItTag(&body, "session_id")
	if err != nil {
		return nil, err
	}

	revision, err := extractReplItTag(&body, "revision_id")
	if err != nil {
		return nil, err
	}

	repl.newURL = fmt.Sprintf("%s/%s/%s", replBaseURL, id, revision)
	return repl, nil
}

func indentC(repl *replData) (*replData, error) {
	if strings.ToLower(repl.language) != "c" {
		return nil, fmt.Errorf("essa linguagem não é C")
	}

	// Magic arguments by @mpaganini: https://github.com/marcopaganini/sock
	args := "--no-tabs --tab-size4 --indent-level4 --braces-on-if-line --cuddle-else --braces-on-func-def-line --braces-on-struct-decl-line --cuddle-do-while --no-space-after-function-call-names --no-space-after-parentheses --dont-break-procedure-type -l666"

	cmd := exec.Command("indent", strings.Split(args, " ")...)
	stdin, _ := cmd.StdinPipe()

	var outbuf, errbuf bytes.Buffer
	cmd.Stdout = &outbuf
	cmd.Stderr = &errbuf

	io.WriteString(stdin, repl.code)
	stdin.Close()

	if err := cmd.Run(); err != nil {
		if len(errbuf.String()) > 0 {
			return nil, fmt.Errorf("erros detectados:\n%s", strings.Replace(errbuf.String(), "indent: Standard input:", "linha ", -1))
		}
		return nil, err
	}

	indented := outbuf.String()
	errors := strings.Replace(errbuf.String(), "indent: Standard input:", "linha ", -1)
	repl.indented = indented
	repl.errors = errors

	return repl, nil
}

func indent(repl *replData) (*replData, error) {
	switch strings.ToLower(repl.language) {
	case "c":
		return indentC(repl)
	default:
		return nil, fmt.Errorf("ainda não sei indentar essa linguagem. Se puder ajudar, faça um pull request para https://github.com/OsProgramadores/osprogramadores_bot :)")
	}
}
