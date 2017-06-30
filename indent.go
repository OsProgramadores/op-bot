package main

import (
	"bytes"
	"errors"
	"fmt"
	"html"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os/exec"
	"strings"
)

type replData struct {
	id       string
	language string
	url      string
	newUrl   string
	code     string
	indented string
	errors   string
}

func parseReplItTag(body []byte, tag string) (string, error) {
	targetS := []byte(fmt.Sprintf("\"%s\":\"", tag))
	targetE := []byte("\"")

	if start := bytes.Index(body, targetS); start != -1 {
		if end := bytes.Index(body[start+len(targetS):], targetE); end != -1 {
			data := string(body[start+len(targetS) : start+len(targetS)+end])
			return data, nil
		}
	}
	return "", errors.New(fmt.Sprintf("Não foi possível ler a informação '%s'", tag))
}

func downloadReplIt(url string) (replData, error) {
	resp, err := http.Get(url)
	if err != nil {
		return replData{}, err
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
	if id, err := parseReplItTag(body, "session_id"); err == nil {
		repl.id = id
		// Language.
		if lang, err := parseReplItTag(body, "language"); err == nil {
			repl.language = lang
			return repl, nil
		}
	}

	return replData{}, errors.New("Não foi possível baixar o código do repl.it")
}

func uploadIndentedCode(repl replData) (replData, error) {
	if len(repl.indented) == 0 {
		return repl, errors.New("Código não foi indentado")
	}

	if repl.code == repl.indented {
		repl.newUrl = repl.url
		return repl, errors.New("Código já estava indentado")
	}

	postUrl := "https://repl.it/save"
	fmt.Printf("%v\n", repl)
	formData := url.Values{"id": {repl.id}, "language": {repl.language}, "editor_text": {repl.indented}}
	resp, err := http.PostForm(postUrl, formData)
	if err != nil {
		//TODO: handle error
		fmt.Println("Oh noes..")
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)

	fmt.Println(string(body))

	if id, err := parseReplItTag(body, "session_id"); err == nil && id == repl.id {
		if revision, err := parseReplItTag(body, "revision_id"); err == nil {
			repl.newUrl = fmt.Sprintf("https://repl.it/%s/%s", id, revision)
			return repl, nil
		}
	}
	//FIXME
	return repl, errors.New("Hmm")
}

func indentC(repl replData) replData {
	if repl.language != "c" {
		return repl
	}

	// Magic arguments by @mpaganini: https://github.com/marcopaganini/sock
	args := fmt.Sprintf("--no-tabs --tab-size4 --indent-level4 --braces-on-if-line --cuddle-else --braces-on-func-def-line --braces-on-struct-decl-line --cuddle-do-while --no-space-after-function-call-names --no-space-after-parentheses --dont-break-procedure-type -l666")

	cmd := exec.Command("indent", strings.Split(args, " ")...)
	stdin, _ := cmd.StdinPipe()

	var outbuf, errbuf bytes.Buffer
	cmd.Stdout = &outbuf
	cmd.Stderr = &errbuf

	go func() {
		defer stdin.Close()
		io.WriteString(stdin, repl.code)
	}()

	cmd.Run()

	indented := outbuf.String()
	errors := strings.Replace(errbuf.String(), "indent: Standard input:", "Linha ", -1)
	repl.indented = indented
	repl.errors = errors

	return repl
}
