package main

import (
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"sync"
)

const (
	locationDb = "location.json"
)

type geoLocation struct {
	Latitude  string
	Longitude string
}

type geoLocationsMutex struct {
	sync.RWMutex
	coords map[string]geoLocation
}

var (
	locations = geoLocationsMutex{coords: make(map[string]geoLocation)}
)

// saveLocations saves all known locations in the geoLocation map into
// the locationDb file.
func saveLocations(m map[string]geoLocation) error {
	buf, err := json.Marshal(m)
	if err != nil {
		return err
	}

	datadir, err := dataDir()
	if err != nil {
		return err
	}

	if _, err = os.Stat(datadir); os.IsNotExist(err) {
		err = os.MkdirAll(datadir, os.ModeDir)
	}
	if err != nil {
		return err
	}

	f := filepath.Join(datadir, locationDb)
	return ioutil.WriteFile(f, buf, 0644)
}

// readLocations reads locations from the locationDb file.
func readLocations() error {
	locations.RLock()
	defer locations.RUnlock()

	datadir, err := dataDir()
	if err != nil {
		return err
	}
	f := filepath.Join(datadir, locationDb)

	buf, err := ioutil.ReadFile(f)
	if err != nil {
		return err
	}

	return json.Unmarshal(buf, &locations.coords)
}

// randomizeCoordinate truncates the lat/long coordinate to one decimal and
// adds noise after the second decimal. This should provide an error radius
// of about 6.9 miles.
func randomizeCoordinate(c float64) string {
	return fmt.Sprintf("%.1f", c+rand.Float64()/1000.0)
}

// handleLocation handles the /location request to the bot.
func handleLocation(key, id string, lat, lon float64) error {
	h := sha1.New()
	io.WriteString(h, fmt.Sprintf("%s%s", key, id))
	userid := fmt.Sprintf("%x", h.Sum(nil))

	locations.Lock()
	defer locations.Unlock()
	locations.coords[userid] = geoLocation{randomizeCoordinate(lat), randomizeCoordinate(lon)}
	return saveLocations(locations.coords)
}

// serveLocations serves the lat/long list in memory in JSON format over HTTP.
// Only (previously obfuscated) lat/long coordinates are served, not user IDs.
func serveLocations(config botConfig) {
	readLocations()
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
