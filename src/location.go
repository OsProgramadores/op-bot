package main

import (
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"io"
	"log"
	"math/rand"
	"net/http"
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
}

// newGeolocations returns a new instance of geoLocations
func newGeolocations(locationKey string) *geoLocations {
	return &geoLocations{
		coords:      map[string]geoLocation{},
		locationKey: locationKey,
		locationDB:  locationDB,
	}
}

// readLocations reads locations from the locationDB file.
func (g *geoLocations) readLocations() error {
	g.RLock()
	defer g.RUnlock()
	return readJSONFromDataDir(&g.coords, g.locationDB)
}

// processLocation handles the /location request to the bot.
func (g *geoLocations) processLocation(id int, lat, lon float64) error {
	h := sha1.New()
	io.WriteString(h, fmt.Sprintf("%s%d", g.locationKey, id))
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
		// Setting the handler to "/" means "match anything under /", so we
		// explicitly test here (this prevents serving twice to browsers due
		// to the automatic request to /favicon.ico.
		if r.URL.Path != "/" {
			log.Printf("Ignoring invalid path request: %s", r.URL.Path)
			http.Error(w, "Handler not found", http.StatusNotFound)
			return
		}

		g.RLock()
		data := []geoLocation{}
		for _, location := range g.coords {
			data = append(data, location)
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

		log.Printf("Served %d lat/long pairs to %s", len(g.coords), r.RemoteAddr)
	})

	http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
}

// processLocationRequest fetches user geo-location information from the
// request and adds the approximate location of the user to a point in the map
// using handleLocation.  Returns a visible message to the user in case of
// problems.
func (x *opBot) processLocationRequest(bot tgbotInterface, update tgbotapi.Update) {
	userid := update.Message.From.ID
	location := update.Message.Location

	err := x.geolocations.processLocation(userid, location.Latitude, location.Longitude)

	// Give feedback to user, if message was sent privately.
	if isPrivateChat(update.Message.Chat) {
		message := T("location_success")
		if err != nil {
			message = T("location_fail")
		}
		sendReply(bot, update.Message.Chat.ID, update.Message.MessageID, message)
	}
}

// randomizeCoordinate truncates the lat/long coordinate to one decimal and
// adds noise after the second decimal. This should provide an error radius
// of about 6.9 miles.
func randomizeCoordinate(c float64) string {
	return fmt.Sprintf("%.1f", c+rand.Float64()/1000.0)
}
