package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
)

/*

Example:

	$ curl 'localhost:8080/weather/?lat=30.489772&lon=-99.771335'
	{"alerts":[],"conditions":["overcast clouds"],"temperature":"moderate"}

Things I would want to do, given more time:

1. Write tests for OWMService using recorded responses
	* Make necessary refactorings so can mock out the HTTP client Get.
2. Write tests for the HTTP server
	* Make OWMService an interface so we can mock that entirely out in tests
3. Split this file up, separating the HTTP server from the service client, etc.

*/

func main() {
	appid := os.Getenv("API_KEY")
	if appid == "" {
		panic("missing (or empty) API_KEY environment variable")
	}

	service := &OWMService{
		client: &http.Client{},
		appid:  appid,
	}

	server := server{
		owm: service,
	}

	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}

	s := &http.Server{
		Addr: addr,
	}
	http.HandleFunc("/weather/", server.weatherHandler)

	log.Printf("Listening on %s\n", addr)
	s.ListenAndServe()
}

type server struct {
	owm *OWMService
}

func (s *server) weatherHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	lat := q.Get("lat")
	lon := q.Get("lon")

	data, err := s.owm.GetWeather(lat, lon)
	if err != nil {
		w.WriteHeader(500)
		msg := fmt.Sprintf("Failed to retrieve weather data: %s", err.Error())
		log.Println(msg)
		w.Write([]byte(msg))
		return
	}

	conditions := make([]string, 0, len(data.Current.Weather))
	for _, cond := range data.Current.Weather {
		conditions = append(conditions, cond.Description)
	}

	var temp string
	tempDegrees := data.Current.FeelsLike
	if tempDegrees < 65 {
		temp = "cold"
	} else if tempDegrees < 80 {
		temp = "moderate"
	} else {
		temp = "hot"
	}

	alerts := make([]string, 0, len(data.Alerts))
	for _, alert := range data.Alerts {
		alerts = append(alerts, alert.Event)
	}

	weather := Weather{
		Alerts:      alerts,
		Conditions:  conditions,
		Temperature: temp,
	}

	json.NewEncoder(w).Encode(&weather)
}

type Weather struct {
	Alerts      []string `json:"alerts"`
	Conditions  []string `json:"conditions"`
	Temperature string   `json:"temperature"`
}

// OWMService is a client for openweathermap.
type OWMService struct {
	client *http.Client
	appid  string
}

func (o *OWMService) GetWeather(lat, lon string) (*OWMApiResponse, error) {
	resp, err := o.client.Get(o.urlFor(lat, lon))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var data OWMApiResponse
	err = json.NewDecoder(resp.Body).Decode(&data)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Error from openweathermap service: %s", data.Message)
	}

	return &data, nil
}

func (o *OWMService) urlFor(lat, lon string) string {
	base, _ := url.Parse("https://api.openweathermap.org/data/2.5/onecall")
	params := url.Values{}
	params.Add("lat", lat)
	params.Add("lon", lon)
	// all we need is 'current' and 'alerts'
	params.Add("exclude", "minutely,hourly,daily")
	params.Add("appid", o.appid)
	params.Add("units", "imperial")
	base.RawQuery = params.Encode()
	return base.String()
}

// OWMApiResponse is a subset of response fields (those that we care about)
// from http://api.openweathermap.org/.
type OWMApiResponse struct {
	Current struct {
		FeelsLike float64 `json:"feels_like"`
		Weather   []struct {
			Description string `json:"description"`
		} `json:"weather"`
	} `json:"current"`
	Alerts []struct {
		Event string `json:"event"`
	} `json:"alerts"`
	Message string `json:"message"`
}
