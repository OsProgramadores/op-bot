package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-telegram-bot-api/telegram-bot-api"
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

// locationHandler receive postal code from user.
func (x *opBot) locationHandler(update tgbotapi.Update) error {
	args := strings.Split(trDelete(update.Message.CommandArguments(), "/-."), " ")
	user := update.Message.From
	cep := ""

	cmd := strings.ToLower(update.Message.Command())

	switch cmd {
	case "setlocation":
		if len(args) != 2 {
			return errors.New(T("setlocation_usage"))
		}
		country := args[0]

		if country != "br" {
			return fmt.Errorf(T("unsupported_country"), args[0])
		}
		cep = args[1]
	case "cep":
		if len(args) != 1 || len(args[0]) == 0 {
			return errors.New(T("missing_postal_code"))
		}
		cep = args[0]
	}

	if err := findCEP(user, cep, x.config); err != nil {
		return fmt.Errorf(T("unable_to_find_location"), cep)
	}

	x.sendReply(update, T("location_success"))
	return nil
}

// API Key from www.cepaberto.com (brazilian postal code to geo location service.)
func findCEP(user *tgbotapi.User, cep string, config botConfig) error {
	url := fmt.Sprintf("http://www.cepaberto.com/api/v2/ceps.json?cep=%s", url.QueryEscape(cep))

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("authorization", `Token token="`+config.CepAbertoKey+`"`)
	client := &http.Client{}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var res cepResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return err
	}

	lat, err := strconv.ParseFloat(res.Latitude, 64)
	if err != nil {
		return errors.New(T("invalid_latitude"))
	}

	long, err := strconv.ParseFloat(res.Longitude, 64)
	if err != nil {
		return errors.New(T("invalid_longitude"))
	}

	return handleLocation(config.LocationKey, fmt.Sprintf("%d", user.ID), lat, long)
}

// trDelete returns a copy of the string with all runes in substring removed.
func trDelete(s, substr string) string {
	ret := bytes.Buffer{}

	for _, r := range s {
		if strings.ContainsRune(substr, r) {
			continue
		}
		ret.WriteRune(r)
	}
	return ret.String()
}
