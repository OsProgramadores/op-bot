package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// Return from www.cepaberto.com
type cepResponse struct {
	CEP        string  `json:"cep"`
	Logradouro string  `json:"logradouro"`
	Bairro     string  `json:"bairro"`
	Cidade     string  `json:"cidade"`
	Estado     string  `json:"estado"`
	Latitude   string  `json:"latitude"`
	Longitude  string  `json:"longitude"`
	Altitude   float64 `json:"altitude"`
	DDD        int     `json:"ddd"`
	IBGE       string  `json:"ibge"`
}

// clean a string
func trDelete(s, substr string) string {
  ret := bytes.Buffer{}

  for _, r := range(s) {
    if strings.ContainsRune(substr, r) {
      continue
    }
    ret.WriteRune(r)
  }
  return ret.String()
}
// locationHandler receive postal code from user.
func locationHandler(update tgbotapi.Update, bot *tgbotapi.BotAPI, config botConfig, msgs botMessages) error {

	args := strings.Split(trDelete(update.Message.CommandArguments(),"/-."), " ")
	user := update.Message.From
	cep := ""

	cmd := strings.ToLower(update.Message.Command())

	switch cmd {
	case "setlocation":
		if len(args) != 2 {
			return errors.New("/setlocation <pais> <código postal>")
		}
		country := args[0]

		if country != "br" {
			return fmt.Errorf("não sei como procurar o Código Postal deste país (%q)", args[0])
		}
		cep = args[1]
	case "cep":
		if len(args) != 1 || len(args[0]) == 0 {
			return errors.New("código postal não especificado")
		}
		cep = args[0]
	}

	if err := findCEP(user, cep, config); err != nil {
		return fmt.Errorf("não foi possível achar a sua localização . CEP %q", cep)
	}

	return errors.New(msgs.LocationSuccess)
}

// API Key from www.cepaberto.com (brazilian postal code to geo location service.)
func findCEP(user *tgbotapi.User, cep string, config botConfig) error {

	url := fmt.Sprintf("http://www.cepaberto.com/api/v2/ceps.json?cep=%s", url.QueryEscape(cep))

	req, err := http.NewRequest("GET", url, nil)

	req.Header.Set("authorization", `Token token="`+config.CepAbertoKey+`"`)
	if err != nil {
		log.Printf("wewRequest: ", err)
		return err
	}

	client := &http.Client{}

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Do: ", err)
		return err
	}

	defer resp.Body.Close()
	var res cepResponse

	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return err
	}

	lat, err := strconv.ParseFloat(res.Latitude, 64)
	if err != nil {
		return fmt.Errorf("latitude Inválida")
	}

	long, err := strconv.ParseFloat(res.Longitude, 64)
	if err != nil {
		return fmt.Errorf("longitude Inválida")
	}

	err = handleLocation(config.LocationKey, fmt.Sprintf("%d", user.ID), lat, long)

	return nil
}
