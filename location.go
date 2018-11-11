package main

import (
	"crypto/sha1"
	"encoding/json"
	"errors"
	"fmt"
	"gopkg.in/telegram-bot-api.v4"
	"io"
	"math/rand"
	"net/http"
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

// geoLocations maps the key -- which is hash from a secret key with the user
// ID -- with the geo location info from this user. It also contains a mutex for
// controlling concurrent access.
type geoLocations struct {
	sync.RWMutex
	coords map[string]geoLocation
	// locationKey contains a key used to scrambled the userIDs when storing the location.
	locationKey string
	locationDB  string
	postal      postalLocatorInterface
}

// postalLocatorInterface provides an interface between locations and the postal code locator.
type postalLocatorInterface interface {
	findPostalLocation(*tgbotapi.User, string) (float64, float64, error)
}

// newGeolocations returns a new instance of geoLocations
func newGeolocations(cepAbertoKey, locationKey string) *geoLocations {
	return &geoLocations{
		coords:      map[string]geoLocation{},
		locationKey: locationKey,
		locationDB:  locationDB,
		postal: &cepLocator{
			cepAbertoKey: cepAbertoKey,
		},
	}
}

// locationHandler receive postal code from user.
func (g *geoLocations) locationHandler(bot tgbotInterface, update tgbotapi.Update) error {
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

	lat, long, err := g.postal.findPostalLocation(user, cep)
	if err != nil {
		return fmt.Errorf(T("unable_to_find_location"), cep)
	}
	if err := g.processLocation(g.locationKey, user.ID, lat, long); err != nil {
		return fmt.Errorf(T("unable_to_find_location"), cep)
	}

	sendReply(bot, update, T("location_success"))
	return nil
}

// readLocations reads locations from the locationDB file.
func (g *geoLocations) readLocations() error {
	g.RLock()
	defer g.RUnlock()
	return readJSONFromDataDir(g.coords, g.locationDB)
}

// processLocation handles the /location request to the bot.
func (g *geoLocations) processLocation(key string, id int, lat, lon float64) error {
	h := sha1.New()
	io.WriteString(h, fmt.Sprintf("%s%d", key, id))
	userid := fmt.Sprintf("%x", h.Sum(nil))

	g.Lock()
	defer g.Unlock()
	g.coords[userid] = geoLocation{randomizeCoordinate(lat), randomizeCoordinate(lon)}
	return safeWriteJSON(g.coords, g.locationDB)
}

// serveLocations serves the lat/long list in memory in JSON format over HTTP.
// Only (previously obfuscated) lat/long coordinates are served, not user IDs.
func (g *geoLocations) serveLocations(port int) {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		g.RLock()
		data := make([]geoLocation, len(g.coords))

		i := 0
		for _, location := range g.coords {
			data[i] = location
			i++
		}
		g.RUnlock()

		js, err := json.Marshal(data)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Write(js)
	})

	http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
}

// randomizeCoordinate truncates the lat/long coordinate to one decimal and
// adds noise after the second decimal. This should provide an error radius
// of about 6.9 miles.
func randomizeCoordinate(c float64) string {
	return fmt.Sprintf("%.1f", c+rand.Float64()/1000.0)
}
