package main

// TODO
// - Add comments on functions.
// - Choose a fixed location for location.json (maybe under ~/.osprogramadores_bot?)
// - Make the port configurable (flag or config option?)

import (
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
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

// This is supposed to run within Lock().
func saveLocations(m map[string]geoLocation) error {
	buf, err := json.Marshal(m)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(locationDb, buf, 0644)
}

func readLocations() error {
	locations.RLock()
	defer locations.RUnlock()

	buf, err := ioutil.ReadFile(locationDb)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(buf, &locations.coords); err != nil {
		return err
	}

	return nil
}

func randomizeCoordinate(c float64) string {
	return fmt.Sprintf("%.1f", c+rand.Float64()/1000.0)
}

func handleLocation(key, id string, lat, lon float64) error {
	h := sha1.New()
	io.WriteString(h, fmt.Sprintf("%s%s", key, id))
	userid := fmt.Sprintf("%x", h.Sum(nil))

	locations.Lock()
	defer locations.Unlock()
	locations.coords[userid] = geoLocation{randomizeCoordinate(lat), randomizeCoordinate(lon)}
	if err := saveLocations(locations.coords); err != nil {
		return err
	}
	return nil
}

func serveLocations() {
	readLocations()
	http.HandleFunc("/", serveLocationsHandler)
	http.ListenAndServe(":3000", nil)
}

// serveLocationsHandler handles external http requests and provides
// a JSON stream of all known geoLocations (without the hash).
func serveLocationsHandler(w http.ResponseWriter, r *http.Request) {
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
}
