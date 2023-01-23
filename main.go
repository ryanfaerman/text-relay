package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func healthCheck(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "ok\n")
}

var (
	accountID = ""
	token     = ""
	defaults  = map[string]string{
		"source":      "guest",
		"destination": "UNKNOWN_DEST",
		"message":     "",
		"type":        "UNKNOWN_TYPE",
	}
	relays = map[string]string{}
)

type apiResponse struct {
	Code   int    `json:"code"`
	Status string `json:"status"`
	Data   string `json:"data"`
	Guid   string `json:"guid"`
}

type apiRequest struct {
	FromDID string `json:"from_did"`
	ToDID   string `json:"to_did"`
	Message string `json:"message"`
}

func receiveText(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "ok \n")

	if err := r.ParseForm(); err != nil {
		log.Error().Err(err).Msg("cannot parse form data")
	}

	for k, v := range defaults {
		if !r.Form.Has(k) {
			r.Form.Set(k, v)
		}
	}

	log.Info().
		Str("from", r.Form.Get("from")).
		Str("to", r.Form.Get("to")).
		Str("type", r.Form.Get("type")).
		Str("msg", r.Form.Get("message")).
		Msg("received text")

	go func(form url.Values) {

		// source MUST be a hardcoded original value
		// destination is looked up in the relays CSV
		// message gets added a postfix (Originally sent FROM: ORIGNAL)

		originalSource := form.Get("from")

		target, ok := relays[form.Get("to")]
		if !ok {
			log.Warn().Str("to", form.Get("to")).Msg("missing relay target")
			return
		}

		form.Set("from", form.Get("to"))
		form.Set("to", target)

		originalMessage := form.Get("message")
		postfixMessage := fmt.Sprintf("Relayed from: %s", originalSource)
		form.Set("message", fmt.Sprintf("%s\n- %s", originalMessage, postfixMessage))

		form.Del("type")

		if token == "" {
			log.Warn().Msg("missing TOKEN, running in log-only mode")
			log.Info().
				Str("from", form.Get("from")).
				Str("to", form.Get("to")).
				Str("msg", form.Get("message")).
				Msg("forwarding message...")

		}

		reqBody := apiRequest{
			FromDID: strings.TrimSpace(form.Get("from")),
			ToDID:   strings.TrimSpace(form.Get("to")),
			Message: form.Get("message"),
		}

		raw, err := json.Marshal(reqBody)
		if err != nil {
			log.Error().Err(err).Msg("cannot marshal post request")
		}

		postReq, err := http.NewRequest(
			"POST",
			fmt.Sprintf("https://api.thinq.com/account/%s/product/origination/sms/send", accountID),
			bytes.NewBuffer(raw),
		)
		if err != nil {
			log.Error().Err(err).Msg("Cannot form post request")
			return
		}
		postReq.Header.Add("Authorization", fmt.Sprintf("Basic %s", token))
		postReq.Header.Add("Content-Type", "application/json")

		//spew.Dump(postReq)
		resp, err := http.DefaultClient.Do(postReq)
		if err != nil {
			log.Error().Err(err).Msg("cannot perform POST")
			return
		}

		if resp.StatusCode != 200 {
			log.Error().Int("status", resp.StatusCode).Msg("API error")
			return
		}

		defer resp.Body.Close()

		var data apiResponse

		if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
			log.Error().Err(err).Msg("cannot decode response")
			return
		}

		if data.Guid == "" {
			log.Error().Msgf("cannot relay; %s", data.Data)
			return
		}

		log.Info().
			Str("from", form.Get("from")).
			Str("to", form.Get("to")).
			Str("msg", form.Get("message")).
			Str("status", data.Status).
			Str("data", data.Data).
			Str("guid", data.Guid).
			Msg("forwarded message")

	}(r.Form)
}

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	token = os.Getenv("TOKEN")
	if token == "" {
		log.Warn().Msg("TOKEN is missing, running in LOG only mode")
	}
	accountID = os.Getenv("ACCOUNT_ID")
	if accountID == "" {
		log.Warn().Msg("ACCOUNT_ID is missing, running in LOG only mode")
	}

	http.HandleFunc("/.well-known/ruok", healthCheck)
	http.HandleFunc("/", receiveText)

	log.Printf("starting text-relay on port %s", port)

	file, err := os.Open("relays.csv")
	if err != nil {
		log.Fatal().Err(err).Msg("cannot read 'relays.csv', maybe create it?")
	}

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		log.Fatal().Err(err).Msg("malformed 'relays.csv', refusing to start")
	}

	for i, row := range records {
		if len(row) != 2 {
			log.Warn().Int("row", i).Msg("malformed row, must have both orignal source and destination")
			continue
		}

		original, target := row[0], row[1]

		malformed := ""

		if original == "" {
			malformed = "original destination is invalid"
		}

		if target == "" {
			malformed = "target destination is invalid"
		}

		if malformed != "" {
			log.Warn().Int("row", i).Msgf("skipping malformed row; %s", malformed)
			continue
		}
		relays[strings.TrimSpace(row[0])] = strings.TrimSpace(row[1])
	}

	if err := http.ListenAndServe(fmt.Sprintf(":%s", port), nil); err != nil {
		log.Fatal().Err(err).Msg("cannot start server")
	}
}
