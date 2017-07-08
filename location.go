package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"sync"
)

const (
	locationDb = "location.json"
)

type Location struct {
	Latitude  string
	Longitude string
}

type Locations struct {
	sync.RWMutex
	location map[string]Location
}

var (
	locations = Locations{location: make(map[string]Location)}
)

// This is supposed to run within Lock().
func saveLocations(m map[string]Location) error {
	buffer := new(bytes.Buffer)
	encoder := gob.NewEncoder(buffer)

	err := encoder.Encode(m)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(locationDb, buffer.Bytes(), 0644)
}

func readLocations() error {
	locations.RLock()
	defer locations.RUnlock()

	f, err := os.Open(locationDb)
	if err != nil {
		return err
	}
	defer f.Close()

	decoder := gob.NewDecoder(f)
	if err := decoder.Decode(&locations.location); err != nil {
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
	userid := string(h.Sum(nil))

	locations.Lock()
	defer locations.Unlock()
	locations.location[userid] = Location{randomizeCoordinate(lat), randomizeCoordinate(lon)}
	if err := saveLocations(locations.location); err != nil {
		return err
	}
	return nil
}

func serveLocations() {
	readLocations()
	http.HandleFunc("/", serveLocationsHandler)
	http.ListenAndServe(":3000", nil)
}

func serveLocationsHandler(w http.ResponseWriter, r *http.Request) {
	locations.RLock()
	data := make([]Location, len(locations.location))

	i := 0
	for _, location := range locations.location {
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
