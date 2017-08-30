package main

import (
	"crypto/sha1"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
)

const (
	locationDB = "location.json"
)

// geoLocation contains the actual location info with both latitude and
// longitude.
type geoLocation struct {
	Latitude  string
	Longitude string
}

// geoLocationList maps the key -- which is hash from a secret key with the user
// ID -- with the geo location info from this user. It also contains a mutex for
// controlling concurrent access.
type geoLocationList struct {
	sync.RWMutex
	coords map[string]geoLocation
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

	if err := findCEP(user, cep, x.config, &x.locations); err != nil {
		return fmt.Errorf(T("unable_to_find_location"), cep)
	}

	x.sendReply(update, T("location_success"))
	return nil
}

// API Key from www.cepaberto.com (brazilian postal code to geo location
// service.)
func findCEP(user *tgbotapi.User, cep string, config botConfig, locations *geoLocationList) error {
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

	return handleLocation(locations, config.LocationKey, fmt.Sprintf("%d", user.ID), lat, long)
}

// readLocations reads locations from the locationDb file.
func readLocations(locations *geoLocationList) error {
	locations.RLock()
	defer locations.RUnlock()

	return readJSONFromDataDir(&locations.coords, locationDB)
}

// randomizeCoordinate truncates the lat/long coordinate to one decimal and
// adds noise after the second decimal. This should provide an error radius
// of about 6.9 miles.
func randomizeCoordinate(c float64) string {
	return fmt.Sprintf("%.1f", c+rand.Float64()/1000.0)
}

// handleLocation handles the /location request to the bot.
func handleLocation(locations *geoLocationList, key, id string, lat, lon float64) error {
	h := sha1.New()
	io.WriteString(h, fmt.Sprintf("%s%s", key, id))
	userid := fmt.Sprintf("%x", h.Sum(nil))

	locations.Lock()
	defer locations.Unlock()
	locations.coords[userid] = geoLocation{randomizeCoordinate(lat), randomizeCoordinate(lon)}
	return safeWriteJSON(locations.coords, locationDB)
}

// serveLocations serves the lat/long list in memory in JSON format over HTTP.
// Only (previously obfuscated) lat/long coordinates are served, not user IDs.
func serveLocations(config botConfig, locations *geoLocationList) {
	readLocations(locations)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		locations.RLock()
		data := make([]geoLocation, len(locations.coords))

		i := 0
		for _, location := range locations.coords {
			data[i] = location
			i++
		}
		locations.RUnlock()

		js, err := json.Marshal(data)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Write(js)
	})

	http.ListenAndServe(fmt.Sprintf(":%d", config.ServerPort), nil)
}
