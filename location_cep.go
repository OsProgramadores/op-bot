package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"gopkg.in/telegram-bot-api.v4"
	"log"
	"net/http"
	"net/url"
	"strconv"
)

// cepLocator provides a findPostalLocation for the Brazilian CEP system.
type cepLocator struct {
	// API key to www.cepaberto.com
	cepAbertoKey string
}

// cepResponse holds the return from www.cepaberto.com.
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

// Find a CEP (Brazilian postal addressing code) using www.cepaberto.com.
func (c *cepLocator) findPostalLocation(user *tgbotapi.User, cep string) (float64, float64, error) {
	url := fmt.Sprintf("http://www.cepaberto.com/api/v2/ceps.json?cep=%s", url.QueryEscape(cep))

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, 0, err
	}

	req.Header.Set("authorization", `Token token="`+c.cepAbertoKey+`"`)
	client := &http.Client{}

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error retrieving CEP: %v\n", err)
		return 0, 0, err
	}
	defer resp.Body.Close()

	var res cepResponse
	if err = json.NewDecoder(resp.Body).Decode(&res); err != nil {
		log.Printf("Error decoding json from www.cepaberto.com: %v\n", err)
		return 0, 0, err
	}

	lat, err := strconv.ParseFloat(res.Latitude, 64)
	if err != nil {
		return 0, 0, errors.New(T("invalid_latitude"))
	}

	long, err := strconv.ParseFloat(res.Longitude, 64)
	if err != nil {
		return 0, 0, errors.New(T("invalid_longitude"))
	}
	return lat, long, nil
}
