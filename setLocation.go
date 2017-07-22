package main

import (
	"fmt"
	"log"
	"strings"
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"encoding/json"
    "net/http"
    "net/url"
	"strconv"
)

type CepResponse struct {
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

	isCep := false
	pais := "??"
	cep := ""
	user := update.Message.From

	if update.Message.Command() == "setlocation" {
		args := strings.Split(strings.Trim(update.Message.CommandArguments(), " "), " ")

		if len(args)!=2 {
			sendReply(update.Message.Chat.ID, update.Message.MessageID, "/setlocation <pais> <código postal>", bot)
			return nil
		}

		pais = args[0]
		cep  = args[1]

		if pais=="br" {
			isCep=true
		}
	} else if update.Message.Command() == "cep" {
		cep = strings.Trim(update.Message.CommandArguments(), " ")
		isCep=true
	} else {
		log.Println("O que estou fazendo aqui comando -> "+update.Message.Command())
		return nil
	}

	if (isCep) {
		cep = strings.Replace(cep,"-","",-1)
		cep = strings.Replace(cep,".","",-1)

		err := procCep(user,cep,config)

		if err == nil {
			if isPrivateChat(update.Message.Chat) {
				message := msgs.LocationSuccess
				if err != nil {
					message = msgs.LocationFail
				}
				sendReply(update.Message.Chat.ID, update.Message.MessageID, message, bot)
			}
		} else {
			sendReply(update.Message.Chat.ID, update.Message.MessageID, "Não foi possível achar a sua localização . CEP "+cep, bot)
		}
	} else {
		sendReply(update.Message.Chat.ID, update.Message.MessageID, "Não sei como procurar o Código Postal deste país ("+pais+")", bot)
	}

	return nil
}

func procCep (user *tgbotapi.User, cep string, config botConfig) error {

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
    var resultado CepResponse

    if err := json.NewDecoder(resp.Body).Decode(&resultado); err != nil {
        return err
    }

	lat,err := strconv.ParseFloat(resultado.Latitude,64)
	if err != nil {
        return fmt.Errorf("Latitude Inválida")
    }

	long,err := strconv.ParseFloat(resultado.Longitude,64)
	if err != nil {
		return fmt.Errorf("Loongitude Inválida")
    }

	err = handleLocation(config.LocationKey, fmt.Sprintf("%d", user.ID), lat , long)

	return nil
}
