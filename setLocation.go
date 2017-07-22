package main

import (
	"errors"
	"encoding/json"
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

// locationHandler receive postal code from user.
func locationHandler(update tgbotapi.Update, bot *tgbotapi.BotAPI, config botConfig, msgs botMessages) error {

	tmp := strings.ToLower(strings.Trim(update.Message.CommandArguments(), " "))
	tmp = strings.Replace(tmp, "-", "", -1)
	tmp = strings.Replace(tmp, ".", "", -1)
	tmp = strings.Replace(tmp, "/", "", -1)
	args := strings.Split(tmp, " ")
	user := update.Message.From
	cep := ""

	cmd := strings.ToLower(update.Message.Command())

	if cmd == "setlocation" && len(args) != 2 {
		return errors.New("/setlocation <pais> <código postal>");
	}

	if cmd == "cep" && len(args) != 1 {
		return errors.New("/cep <código postal>");
	}

	if  (cmd == "setlocation" && args[0] == "br") || cmd == "cep" {
		if cmd == "setlocation" {
			cep = args[1]
		} else {
			cep = args[0]
		}

		if err := findCEP(user,cep,config); err != nil {
			return errors.New("Não foi possível achar a sua localização . CEP "+cep);
		}

		return errors.New(msgs.LocationSuccess);
	}

	return errors.New("Não sei como procurar o Código Postal deste país ("+args[0]+")");
}

func findCEP(user *tgbotapi.User, cep string, config botConfig) error {

	cepSeguro := url.QueryEscape(cep)

	url := fmt.Sprintf("http://www.cepaberto.com/api/v2/ceps.json?cep=%s", cepSeguro)

	req, err := http.NewRequest("GET", url, nil)

	req.Header.Set("Authorization", `Token token="`+config.CepAbertoKey+`"`)
	if err != nil {
		log.Printf("NewRequest: ", err)
		return err
	}

	client := &http.Client{}

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Do: ", err)
		return err
	}

	defer resp.Body.Close()
	var resultado cepResponse

	if err := json.NewDecoder(resp.Body).Decode(&resultado); err != nil {
		return err
	}

	lat, err := strconv.ParseFloat(resultado.Latitude, 64)
	if err != nil {
		return fmt.Errorf("Latitude Inválida")
	}

	long, err := strconv.ParseFloat(resultado.Longitude, 64)
	if err != nil {
		return fmt.Errorf("Loongitude Inválida")
	}

	err = handleLocation(config.LocationKey, fmt.Sprintf("%d", user.ID), lat, long)

	return nil
}
